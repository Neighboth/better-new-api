// Portions adapted from github.com/jasonxu114514/opencode2api (GPL-3.0) by
// jasonxu114514. See THIRD-PARTY-LICENSES.md.
package opencode

import (
	"bufio"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	utls "github.com/refraction-networking/utls"
)

// bunClientHelloSpec reproduces the BoringSSL TLS ClientHello emitted by Bun
// (JA4 t13d1713h1): TLS 1.3 + 1.2, 17 cipher suites, 13 extensions, no GREASE,
// ALPN ["http/1.1"], and X25519MLKEM768 + X25519 key shares.
func bunClientHelloSpec() utls.ClientHelloSpec {
	const x25519MLKEM768 = utls.CurveID(0x11ec)
	return utls.ClientHelloSpec{
		TLSVersMax: utls.VersionTLS13,
		TLSVersMin: utls.VersionTLS12,
		CipherSuites: []uint16{
			utls.TLS_AES_128_GCM_SHA256,
			utls.TLS_AES_256_GCM_SHA384,
			utls.TLS_CHACHA20_POLY1305_SHA256,
			utls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			utls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			utls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			utls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			utls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			utls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
			utls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
			utls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			utls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			utls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			utls.TLS_RSA_WITH_AES_128_GCM_SHA256,
			utls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			utls.TLS_RSA_WITH_AES_128_CBC_SHA,
			utls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
		CompressionMethods: []uint8{0x00},
		Extensions: []utls.TLSExtension{
			&utls.SNIExtension{},
			&utls.ExtendedMasterSecretExtension{},
			&utls.RenegotiationInfoExtension{Renegotiation: utls.RenegotiateOnceAsClient},
			&utls.SupportedCurvesExtension{Curves: []utls.CurveID{
				x25519MLKEM768,
				utls.X25519,
				utls.CurveP256,
				utls.CurveP384,
			}},
			&utls.SupportedPointsExtension{SupportedPoints: []uint8{0x00}},
			&utls.SessionTicketExtension{},
			&utls.ALPNExtension{AlpnProtocols: []string{"http/1.1"}},
			&utls.StatusRequestExtension{},
			&utls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: []utls.SignatureScheme{
				utls.ECDSAWithP256AndSHA256,
				utls.PSSWithSHA256,
				utls.PKCS1WithSHA256,
				utls.ECDSAWithP384AndSHA384,
				utls.PSSWithSHA384,
				utls.PKCS1WithSHA384,
				utls.PSSWithSHA512,
				utls.PKCS1WithSHA512,
				utls.PKCS1WithSHA1,
			}},
			&utls.SCTExtension{},
			&utls.KeyShareExtension{KeyShares: []utls.KeyShare{
				{Group: x25519MLKEM768},
				{Group: utls.X25519},
			}},
			&utls.PSKKeyExchangeModesExtension{Modes: []uint8{utls.PskModeDHE}},
			&utls.SupportedVersionsExtension{Versions: []uint16{
				utls.VersionTLS13,
				utls.VersionTLS12,
			}},
		},
	}
}

// bunCommonHeaders matches Bun's canonical common header table order.
var bunCommonHeaders = []string{
	"accept",
	"authorization",
	"content-type",
}

// formatBunHTTP1Request serializes an HTTP/1.1 request using Bun's exact header
// casing and wire order: common headers first, custom headers sorted lowercase,
// then Bun's built-in Connection, User-Agent, Accept, Host, Accept-Encoding,
// and Content-Length tail.
func formatBunHTTP1Request(req *http.Request, body []byte) []byte {
	method := req.Method
	if method == "" {
		method = http.MethodPost
	}
	target := req.URL.RequestURI()
	if target == "" {
		target = "/"
	}
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}

	lowerMap := make(map[string][]string, len(req.Header))
	for k, vals := range req.Header {
		lk := strings.ToLower(strings.TrimSpace(k))
		if lk == "" {
			continue
		}
		lowerMap[lk] = append(lowerMap[lk], vals...)
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s %s HTTP/1.1\r\n", method, target)

	emitted := map[string]bool{
		"host":            true,
		"connection":      true,
		"user-agent":      true,
		"accept-encoding": true,
		"content-length":  true,
	}

	// 1. Bun common headers (lowercase keys)
	for _, key := range bunCommonHeaders {
		if key == "accept" {
			// Accept is emitted later unless caller explicitly set a non-wildcard Accept
			if vals := lowerMap[key]; len(vals) > 0 && vals[0] != "*/*" {
				for _, v := range vals {
					fmt.Fprintf(&buf, "%s: %s\r\n", key, v)
				}
				emitted[key] = true
			}
			continue
		}
		if vals := lowerMap[key]; len(vals) > 0 {
			for _, v := range vals {
				fmt.Fprintf(&buf, "%s: %s\r\n", key, v)
			}
			emitted[key] = true
		}
	}

	// 2. Remaining custom headers sorted alphabetically (preserve lowercase x-* as Bun does)
	customKeys := make([]string, 0, len(lowerMap))
	for k := range lowerMap {
		if emitted[k] {
			continue
		}
		customKeys = append(customKeys, k)
	}
	sort.Strings(customKeys)
	for _, k := range customKeys {
		wireName := k
		if k == "x-session-id" {
			wireName = "X-Session-Id"
		}
		for _, v := range lowerMap[k] {
			fmt.Fprintf(&buf, "%s: %s\r\n", wireName, v)
		}
	}

	// 3. Bun trailing transport headers
	buf.WriteString("Connection: keep-alive\r\n")
	ua := req.Header.Get("User-Agent")
	if ua == "" {
		ua = opencodeCLIUserAgent
	}
	fmt.Fprintf(&buf, "User-Agent: %s\r\n", ua)
	if !emitted["accept"] {
		buf.WriteString("Accept: */*\r\n")
	}
	fmt.Fprintf(&buf, "Host: %s\r\n", host)
	buf.WriteString("Accept-Encoding: gzip, deflate\r\n")
	if len(body) > 0 || method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
		fmt.Fprintf(&buf, "Content-Length: %d\r\n", len(body))
	}
	buf.WriteString("\r\n")
	if len(body) > 0 {
		buf.Write(body)
	}
	return buf.Bytes()
}

// doBunHTTPRequest sends req over a fresh Bun-fingerprinted TLS + HTTP/1.1
// connection (direct or via HTTP/SOCKS5 proxy).
func doBunHTTPRequest(ctx context.Context, req *http.Request, body []byte, rawProxyURL string) (*http.Response, error) {
	if req == nil || req.URL == nil {
		return nil, errors.New("nil request or URL")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	host := req.URL.Hostname()
	port := req.URL.Port()
	isHTTPS := strings.EqualFold(req.URL.Scheme, "https")
	if port == "" {
		if isHTTPS {
			port = "443"
		} else {
			port = "80"
		}
	}
	targetAddr := net.JoinHostPort(host, port)

	rawConn, err := dialWithOptionalProxy(ctx, targetAddr, rawProxyURL)
	if err != nil {
		return nil, err
	}

	var conn net.Conn = rawConn
	if isHTTPS {
		uConn := utls.UClient(rawConn, &utls.Config{
			ServerName: host,
			NextProtos: []string{"http/1.1"},
		}, utls.HelloCustom)
		spec := bunClientHelloSpec()
		if err := uConn.ApplyPreset(&spec); err != nil {
			_ = rawConn.Close()
			return nil, fmt.Errorf("apply utls bun preset: %w", err)
		}
		if err := uConn.HandshakeContext(ctx); err != nil {
			_ = rawConn.Close()
			return nil, fmt.Errorf("utls handshake failed: %w", err)
		}
		conn = uConn
	}

	wire := formatBunHTTP1Request(req, body)
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetWriteDeadline(deadline)
	} else {
		_ = conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	}
	if _, err := conn.Write(wire); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("write request failed: %w", err)
	}
	_ = conn.SetWriteDeadline(time.Time{})

	// Respect context cancellation while reading response headers
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetReadDeadline(deadline)
	} else {
		_ = conn.SetReadDeadline(time.Now().Add(180 * time.Second))
	}
	br := bufio.NewReaderSize(conn, 32*1024)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("read response failed: %w", err)
	}
	_ = conn.SetReadDeadline(time.Time{})

	// Wrap body so closing it closes the underlying socket, and decode gzip/deflate transparently.
	resp.Body = wrapBunResponseBody(resp, conn)
	return resp, nil
}

type connClosingReader struct {
	io.Reader
	closers []io.Closer
}

func (c *connClosingReader) Close() error {
	var firstErr error
	for _, cl := range c.closers {
		if cl != nil {
			if err := cl.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func wrapBunResponseBody(resp *http.Response, conn net.Conn) io.ReadCloser {
	encoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	switch encoding {
	case "gzip":
		gr, err := gzip.NewReader(resp.Body)
		if err == nil {
			resp.Header.Del("Content-Encoding")
			resp.Header.Del("Content-Length")
			resp.ContentLength = -1
			return &connClosingReader{
				Reader:  gr,
				closers: []io.Closer{gr, resp.Body, conn},
			}
		}
	case "deflate":
		fr := flate.NewReader(resp.Body)
		resp.Header.Del("Content-Encoding")
		resp.Header.Del("Content-Length")
		resp.ContentLength = -1
		return &connClosingReader{
			Reader:  fr,
			closers: []io.Closer{fr, resp.Body, conn},
		}
	}
	return &connClosingReader{
		Reader:  resp.Body,
		closers: []io.Closer{resp.Body, conn},
	}
}

func dialWithOptionalProxy(ctx context.Context, targetAddr string, rawProxyURL string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 25 * time.Second, KeepAlive: 30 * time.Second}
	trimmed := strings.TrimSpace(rawProxyURL)
	if trimmed == "" {
		return dialer.DialContext(ctx, "tcp", targetAddr)
	}
	proxyURL, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}
	scheme := strings.ToLower(proxyURL.Scheme)
	switch scheme {
	case "socks5", "socks5h":
		return dialSOCKS5(ctx, dialer, proxyURL, targetAddr)
	case "http", "https":
		return dialHTTPConnect(ctx, dialer, proxyURL, targetAddr)
	default:
		return dialer.DialContext(ctx, "tcp", targetAddr)
	}
}

func dialHTTPConnect(ctx context.Context, dialer *net.Dialer, proxyURL *url.URL, targetAddr string) (net.Conn, error) {
	proxyAddr := proxyURL.Host
	if proxyURL.Port() == "" {
		if strings.EqualFold(proxyURL.Scheme, "https") {
			proxyAddr = net.JoinHostPort(proxyURL.Hostname(), "443")
		} else {
			proxyAddr = net.JoinHostPort(proxyURL.Hostname(), "80")
		}
	}
	conn, err := dialer.DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("dial http proxy %s: %w", proxyAddr, err)
	}
	var reqBuf bytes.Buffer
	fmt.Fprintf(&reqBuf, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n", targetAddr, targetAddr)
	if proxyURL.User != nil {
		user := proxyURL.User.Username()
		pass, _ := proxyURL.User.Password()
		cred := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
		fmt.Fprintf(&reqBuf, "Proxy-Authorization: Basic %s\r\n", cred)
	}
	reqBuf.WriteString("\r\n")
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	if _, err := conn.Write(reqBuf.Bytes()); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("write CONNECT to proxy: %w", err)
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("read CONNECT response: %w", err)
	}
	_ = resp.Body.Close()
	_ = conn.SetDeadline(time.Time{})
	if resp.StatusCode != http.StatusOK {
		_ = conn.Close()
		return nil, fmt.Errorf("proxy CONNECT rejected with status %d", resp.StatusCode)
	}
	return conn, nil
}

func dialSOCKS5(ctx context.Context, dialer *net.Dialer, proxyURL *url.URL, targetAddr string) (net.Conn, error) {
	proxyAddr := proxyURL.Host
	if proxyURL.Port() == "" {
		proxyAddr = net.JoinHostPort(proxyURL.Hostname(), "1080")
	}
	conn, err := dialer.DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("dial socks5 proxy %s: %w", proxyAddr, err)
	}
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))

	hasAuth := proxyURL.User != nil
	if hasAuth {
		_, err = conn.Write([]byte{0x05, 0x02, 0x00, 0x02})
	} else {
		_, err = conn.Write([]byte{0x05, 0x01, 0x00})
	}
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("socks5 greeting write: %w", err)
	}
	greetResp := make([]byte, 2)
	if _, err := io.ReadFull(conn, greetResp); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("socks5 greeting read: %w", err)
	}
	if greetResp[0] != 0x05 {
		_ = conn.Close()
		return nil, fmt.Errorf("socks5 invalid version %d", greetResp[0])
	}
	if greetResp[1] == 0x02 && hasAuth {
		user := proxyURL.User.Username()
		pass, _ := proxyURL.User.Password()
		authReq := make([]byte, 0, 3+len(user)+len(pass))
		authReq = append(authReq, 0x01, byte(len(user)))
		authReq = append(authReq, []byte(user)...)
		authReq = append(authReq, byte(len(pass)))
		authReq = append(authReq, []byte(pass)...)
		if _, err := conn.Write(authReq); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("socks5 auth write: %w", err)
		}
		authResp := make([]byte, 2)
		if _, err := io.ReadFull(conn, authResp); err != nil || authResp[1] != 0x00 {
			_ = conn.Close()
			return nil, errors.New("socks5 authentication failed")
		}
	} else if greetResp[1] != 0x00 {
		_ = conn.Close()
		return nil, fmt.Errorf("socks5 unsupported auth method %d", greetResp[1])
	}

	targetHost, targetPortStr, err := net.SplitHostPort(targetAddr)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	portNum, err := strconv.Atoi(targetPortStr)
	if err != nil || portNum <= 0 || portNum > 65535 {
		_ = conn.Close()
		return nil, fmt.Errorf("invalid target port %q", targetPortStr)
	}
	if len(targetHost) > 255 {
		_ = conn.Close()
		return nil, errors.New("target hostname too long for socks5")
	}

	connectReq := make([]byte, 0, 7+len(targetHost))
	connectReq = append(connectReq, 0x05, 0x01, 0x00, 0x03, byte(len(targetHost)))
	connectReq = append(connectReq, []byte(targetHost)...)
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(portNum))
	connectReq = append(connectReq, portBytes...)
	if _, err := conn.Write(connectReq); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("socks5 connect write: %w", err)
	}

	hdr := make([]byte, 4)
	if _, err := io.ReadFull(conn, hdr); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("socks5 connect read: %w", err)
	}
	if hdr[1] != 0x00 {
		_ = conn.Close()
		return nil, fmt.Errorf("socks5 connect failed with code %d", hdr[1])
	}
	switch hdr[3] {
	case 0x01:
		_, err = io.ReadFull(conn, make([]byte, 4+2))
	case 0x03:
		lenBuf := make([]byte, 1)
		if _, err = io.ReadFull(conn, lenBuf); err == nil {
			_, err = io.ReadFull(conn, make([]byte, int(lenBuf[0])+2))
		}
	case 0x04:
		_, err = io.ReadFull(conn, make([]byte, 16+2))
	default:
		err = fmt.Errorf("socks5 unknown atyp %d", hdr[3])
	}
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("socks5 bound addr read: %w", err)
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}
