package jevclient

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/kataras/jev"
)

const defaultBaseURL = "https://api.typesafe.ai"

func validateBaseURL(raw string) (string, error) {
	if raw == "" {
		return defaultBaseURL, nil
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
