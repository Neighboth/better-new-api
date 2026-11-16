package common

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

func init() {
	proxy.RegisterDialerType("socks4", newSOCKS4)
	proxy.RegisterDialerType("socks4a", newSOCKS4a)
}

type socks4Dialer struct {
	proxyNetwork string
	proxyAddress string
	user         string
	isSOCKS4a    bool
	forward      proxy.Dialer
}

func newSOCKS4(u *url.URL, forward proxy.Dialer) (proxy.Dialer, error) {
	return newSOCKS4Internal(u, forward, false)
}

func newSOCKS4a(u *url.URL, forward proxy.Dialer) (proxy.Dialer, error) {
	return newSOCKS4Internal(u, forward, true)
}

func newSOCKS4Internal(u *url.URL, forward proxy.Dialer, isSOCKS4a bool) (proxy.Dialer, error) {
	var user string
	if u.User != nil {
		user = u.User.Username()
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		host = net.JoinHostPort(host, "1080")
	}
	return &socks4Dialer{
		proxyNetwork: "tcp",
		proxyAddress: host,
		user:         user,
		isSOCKS4a:    isSOCKS4a,
		forward:      forward,
	}, nil
}

func (s *socks4Dialer) Dial(network, addr string) (net.Conn, error) {
	return s.DialContext(context.Background(), network, addr)
}

func (s *socks4Dialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, fmt.Errorf("socks4: unsupported network: %s", network)
	}

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("socks4: invalid target address %s: %w", addr, err)
	}

	portNum, err := strconv.Atoi(portStr)
	if err != nil || portNum < 1 || portNum > 65535 {
		return nil, fmt.Errorf("socks4: invalid target port: %s", portStr)
	}

	var proxyConn net.Conn
	if ctxDialer, ok := s.forward.(proxy.ContextDialer); ok {
		proxyConn, err = ctxDialer.DialContext(ctx, s.proxyNetwork, s.proxyAddress)
	} else {
		proxyConn, err = s.forward.Dial(s.proxyNetwork, s.proxyAddress)
	}
	if err != nil {
		return nil, fmt.Errorf("socks4: failed to connect to proxy %s: %w", s.proxyAddress, err)
	}

	if deadline, ok := ctx.Deadline(); ok {
		_ = proxyConn.SetDeadline(deadline)
		defer func() {
			_ = proxyConn.SetDeadline(time.Time{})
		}()
	}

	// SOCKS4 request:
	// VN (1 byte: 0x04) + CD (1 byte: 0x01 CONNECT) + DSTPORT (2 bytes) + DSTIP (4 bytes) + USERID (variable) + 0x00
	// If SOCKS4a and host is not an IPv4: DSTIP is 0.0.0.1, followed by USERID + 0x00 + HOSTNAME + 0x00
	req := make([]byte, 0, 128)
	req = append(req, 0x04, 0x01)

	// DSTPORT (big endian uint16)
	portBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(portBuf, uint16(portNum))
	req = append(req, portBuf...)

	ip := net.ParseIP(host)
	ip4 := ip.To4()

	if ip4 != nil {
		req = append(req, ip4...)
		req = append(req, []byte(s.user)...)
		req = append(req, 0x00)
	} else if s.isSOCKS4a {
		// SOCKS4a allows domain name resolution on proxy side: IP is 0.0.0.1 (or 0.0.0.x with x != 0)
		req = append(req, 0x00, 0x00, 0x00, 0x01)
		req = append(req, []byte(s.user)...)
		req = append(req, 0x00)
		req = append(req, []byte(host)...)
		req = append(req, 0x00)
	} else {
		// Resolve locally for standard SOCKS4
		resolvedIPs, resolveErr := net.LookupIP(host)
		if resolveErr != nil {
			_ = proxyConn.Close()
			return nil, fmt.Errorf("socks4: failed to resolve target host %s: %w", host, resolveErr)
		}
		var foundIPv4 net.IP
		for _, rip := range resolvedIPs {
			if r4 := rip.To4(); r4 != nil {
				foundIPv4 = r4
				break
			}
		}
		if foundIPv4 == nil {
			_ = proxyConn.Close()
			return nil, fmt.Errorf("socks4: no IPv4 address found for %s", host)
		}
		req = append(req, foundIPv4...)
		req = append(req, []byte(s.user)...)
		req = append(req, 0x00)
	}

	if _, err := proxyConn.Write(req); err != nil {
		_ = proxyConn.Close()
		return nil, fmt.Errorf("socks4: handshake write failed: %w", err)
	}

	// Read 8-byte reply:
	// VN (1 byte) + CD (1 byte: 90 = granted) + DSTPORT (2 bytes) + DSTIP (4 bytes)
	resp := make([]byte, 8)
	if _, err := io.ReadFull(proxyConn, resp); err != nil {
		_ = proxyConn.Close()
		return nil, fmt.Errorf("socks4: handshake read failed: %w", err)
	}

	if resp[1] != 90 { // 90 = Request granted
		_ = proxyConn.Close()
		var reason string
		switch resp[1] {
		case 91:
			reason = "request rejected or failed"
		case 92:
			reason = "request rejected: SOCKS server cannot connect to identd"
		case 93:
			reason = "request rejected: identd report mismatch"
		default:
			reason = fmt.Sprintf("status code %d", resp[1])
		}
		return nil, fmt.Errorf("socks4: connection rejected: %s", reason)
	}

	return proxyConn, nil
}
