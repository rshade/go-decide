// Package clientkit holds the HTTP guarantees shared by the Jev and clef
// clients: a validated base URL, a client that never follows redirects, a
// logger that redacts the credential, and the opt-in response cache.
package clientkit

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/kataras/jev"
)

// ValidateBaseURL returns raw, or def when raw is empty. A base URL must be
// https, or http to a loopback IP address, and must not carry credentials, a
// query or a fragment.
func ValidateBaseURL(raw, def string) (string, error) {
	if raw == "" {
		return def, nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("%w: base URL must be an absolute https URL, got %q", jev.ErrConfig, raw)
	}
	if u.User != nil {
		return "", fmt.Errorf("%w: base URL must not include credentials", jev.ErrConfig)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("%w: base URL must not include a query or fragment", jev.ErrConfig)
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !isLoopback(u.Hostname()) {
			return "", fmt.Errorf("%w: http base URL is only allowed for loopback hosts, got %q", jev.ErrConfig, u.Hostname())
		}
	default:
		return "", fmt.Errorf("%w: base URL scheme must be https, got %q", jev.ErrConfig, u.Scheme)
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return u.String(), nil
}

func isLoopback(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// NoRedirect is the CheckRedirect policy that returns the redirect response
// itself, so a bearer token is never sent to another host.
func NoRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
