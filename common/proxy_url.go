package common

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// NormalizeProxyLine converts any standard proxy format (including host:port:user:pass,
// socks5://host:port:user:pass, user:pass@host:port, or raw host:port) into a
// valid normalized URL string with properly escaped credentials.
func NormalizeProxyLine(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("empty proxy URL")
	}

	scheme := "socks5"
	rest := trimmed

	// Check if scheme prefix is present
	if idx := strings.Index(rest, "://"); idx != -1 {
		scheme = strings.ToLower(rest[:idx])
		rest = rest[idx+3:]
	}

	var host, port, user, pass string

	// Case A: contains '@' -> user:pass@host:port (split on LAST '@' in case pass contains '@')
	if atIdx := strings.LastIndex(rest, "@"); atIdx != -1 {
		userinfo := rest[:atIdx]
		hostport := rest[atIdx+1:]

		if colonIdx := strings.Index(userinfo, ":"); colonIdx != -1 {
			user = userinfo[:colonIdx]
			pass = userinfo[colonIdx+1:]
		} else {
			user = userinfo
		}

		if strings.HasPrefix(hostport, "[") {
			closeBracket := strings.Index(hostport, "]")
			if closeBracket == -1 {
				return "", fmt.Errorf("invalid IPv6 address in proxy URL: missing closing bracket")
			}
			host = hostport[1:closeBracket]
			after := hostport[closeBracket+1:]
			if strings.HasPrefix(after, ":") {
				port = after[1:]
			}
		} else if strings.Count(hostport, ":") > 1 {
			if ip := net.ParseIP(hostport); ip != nil {
				host = hostport
			} else if h, p, err := net.SplitHostPort(hostport); err == nil {
				host = h
				port = p
			} else {
				host = hostport
			}
		} else if colonIdx := strings.LastIndex(hostport, ":"); colonIdx != -1 {
			host = hostport[:colonIdx]
			port = hostport[colonIdx+1:]
		} else {
			host = hostport
		}
	} else if strings.HasPrefix(rest, "[") {
		// Case B: Bracketed IPv6, e.g. [2001:db8::1]:1080:user:pass or [2001:db8::1]:1080 or [2001:db8::1]
		closeBracket := strings.Index(rest, "]")
		if closeBracket == -1 {
			return "", fmt.Errorf("invalid IPv6 address in proxy URL: missing closing bracket")
		}
		host = rest[1:closeBracket]
		after := rest[closeBracket+1:]
		if strings.HasPrefix(after, ":") {
			parts := strings.Split(after[1:], ":")
			if len(parts) >= 3 {
				port = parts[0]
				user = parts[1]
				pass = strings.Join(parts[2:], ":")
			} else if len(parts) == 2 {
				port = parts[0]
				user = parts[1]
			} else if len(parts) == 1 {
				port = parts[0]
			}
		}
	} else if ip := net.ParseIP(rest); ip != nil {
		// Bare IPv6 address
		host = rest
	} else {
		// Case C: host:port:user:pass or host:port
		parts := strings.Split(rest, ":")
		if len(parts) >= 4 {
			host = parts[0]
			port = parts[1]
			user = parts[2]
			pass = strings.Join(parts[3:], ":")
		} else if len(parts) == 2 {
			host = parts[0]
			port = parts[1]
		} else if len(parts) == 1 {
			host = parts[0]
		} else {
			return "", fmt.Errorf("invalid proxy format: %s", raw)
		}
	}

	host = strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(host), "]"), "[")

	if host == "" {
		return "", fmt.Errorf("proxy URL must include a host")
	}

	if port == "" {
		if scheme == "socks5" || scheme == "socks5h" {
			port = "1080"
		} else if scheme == "https" {
			port = "443"
		} else if scheme == "http" {
			port = "80"
		} else {
			port = "8080"
		}
	} else {
		p, err := strconv.Atoi(port)
		if err != nil || p < 1 || p > 65535 {
			return "", fmt.Errorf("proxy URL must include a valid port")
		}
	}

	u := &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(host, port),
	}
	if user != "" || pass != "" {
		u.User = url.UserPassword(user, pass)
	}
	return u.String(), nil
}

// ParseProxyURLStrict validates and normalizes a proxy URL for persistence.
func ParseProxyURLStrict(rawProxyURL string) (*url.URL, error) {
	parsedURL, _, err := parseProxyURL(rawProxyURL, false)
	return parsedURL, err
}

// ParseProxyURLRuntime validates and normalizes a proxy URL for runtime use.
// The boolean result reports whether a legacy path, query, or fragment was removed.
func ParseProxyURLRuntime(rawProxyURL string) (*url.URL, bool, error) {
	return parseProxyURL(rawProxyURL, true)
}

func parseProxyURL(rawProxyURL string, allowLegacySuffix bool) (*url.URL, bool, error) {
	trimmedProxyURL := strings.TrimSpace(rawProxyURL)
	if trimmedProxyURL == "" {
		return nil, false, nil
	}

	// Try standard url.Parse first
	parsedURL, err := url.Parse(trimmedProxyURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Hostname() == "" {
		// Fallback to NormalizeProxyLine for non-standard formats (e.g. host:port:user:pass)
		normalized, normErr := NormalizeProxyLine(trimmedProxyURL)
		if normErr != nil {
			return nil, false, fmt.Errorf("invalid proxy URL")
		}
		parsedURL, err = url.Parse(normalized)
		if err != nil {
			return nil, false, fmt.Errorf("invalid proxy URL")
		}
	}
	parsedURL.Scheme = strings.ToLower(parsedURL.Scheme)
	switch parsedURL.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, false, fmt.Errorf("proxy URL must use http, https, socks5, or socks5h")
	}
	if parsedURL.Hostname() == "" {
		return nil, false, fmt.Errorf("proxy URL must include a host")
	}
	if portText := parsedURL.Port(); portText != "" {
		port, err := strconv.Atoi(portText)
		if err != nil || port < 1 || port > 65535 {
			return nil, false, fmt.Errorf("proxy URL must include a valid port")
		}
	}

	hasQuery := parsedURL.RawQuery != "" || parsedURL.ForceQuery
	hasFragment := strings.Contains(trimmedProxyURL, "#")
	escapedPath := parsedURL.EscapedPath()
	hasNonRootPath := escapedPath != "" && escapedPath != "/"
	legacySuffixStripped := hasQuery || hasFragment || hasNonRootPath
	if !allowLegacySuffix {
		switch {
		case hasQuery:
			return nil, false, fmt.Errorf("proxy URL must not include a query")
		case hasFragment:
			return nil, false, fmt.Errorf("proxy URL must not include a fragment")
		case hasNonRootPath:
			return nil, false, fmt.Errorf("proxy URL must not include a path")
		}
	}

	parsedURL.Path = ""
	parsedURL.RawPath = ""
	parsedURL.RawQuery = ""
	parsedURL.ForceQuery = false
	parsedURL.Fragment = ""
	parsedURL.RawFragment = ""

	if (parsedURL.Scheme == "socks5" || parsedURL.Scheme == "socks5h") && parsedURL.Port() == "" {
		parsedURL.Host = net.JoinHostPort(parsedURL.Hostname(), "1080")
	}

	return parsedURL, legacySuffixStripped, nil
}
