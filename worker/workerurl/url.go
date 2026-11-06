// Package workerurl validates the coordinator origin before sending secrets.
package workerurl

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

// Validate allows HTTPS and local-only HTTP for development. Userinfo, query
// strings and fragments are prohibited so credentials cannot be redirected.
func Validate(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("invalid BYOG server URL")
	}
	if strings.EqualFold(u.Scheme, "https") {
		return nil
	}
	if strings.EqualFold(u.Scheme, "http") {
		host := u.Hostname()
		if strings.EqualFold(host, "localhost") {
			return nil
		}
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			return nil
		}
	}
	return errors.New("BYOG requires HTTPS except for loopback development")
}
