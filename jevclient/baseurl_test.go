package jevclient

import (
	"errors"
	"testing"

	"github.com/kataras/jev"
)

func TestValidateBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "empty uses default", raw: "", want: "https://api.typesafe.ai"},
		{name: "https remote", raw: "https://gateway.example.com/v1", want: "https://gateway.example.com/v1"},
		{name: "trailing slash trimmed", raw: "https://api.typesafe.ai/", want: "https://api.typesafe.ai"},
		{name: "http loopback ipv4", raw: "http://127.0.0.1:8080", want: "http://127.0.0.1:8080"},
		{name: "http loopback ipv6", raw: "http://[::1]:8080", want: "http://[::1]:8080"},
		{name: "http loopback range", raw: "http://127.0.0.2:8080", want: "http://127.0.0.2:8080"},
		{name: "http localhost name refused", raw: "http://localhost:8080", wantErr: true},
		{name: "http remote refused", raw: "http://example.com", wantErr: true},
		{name: "http private address refused", raw: "http://10.0.0.5", wantErr: true},
		{name: "unsupported scheme", raw: "ftp://example.com", wantErr: true},
		{name: "no host", raw: "https://", wantErr: true},
		{name: "unparseable", raw: "://bad", wantErr: true},
		{name: "credentials refused", raw: "https://user:pw@example.com", wantErr: true},
		{name: "query refused", raw: "https://example.com?x=1", wantErr: true},
		{name: "fragment refused", raw: "https://example.com#frag", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateBaseURL(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("validateBaseURL(%q) = %q, want error", tt.raw, got)
				}
				if !errors.Is(err, jev.ErrConfig) {
					t.Fatalf("validateBaseURL(%q) error %v does not wrap jev.ErrConfig", tt.raw, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateBaseURL(%q) unexpected error: %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("validateBaseURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
