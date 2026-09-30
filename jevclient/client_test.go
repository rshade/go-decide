package jevclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/kataras/jev"
)

const noulBody = `{"model":"jev-latest","answers":{"safe":{"type":"noul","noul":0.9}},` +
	`"usage":{"input_tokens":5,"output_tokens":1}}`

func setEnv(t *testing.T, key, baseURL string) {
	t.Helper()
	t.Setenv("TYPESAFE_API_KEY", key)
	t.Setenv("TYPESAFE_BASE_URL", baseURL)
	t.Setenv("TYPESAFE_DEFAULT_MODEL", "")
	t.Setenv("TYPESAFE_LOG_LEVEL", "")
}

func safeRequest() jev.Request {
	return jev.Request{
		State:     "hello",
		Questions: jev.Questions{"safe": jev.Noul{Instructions: "Is this text safe?"}},
	}
}

func newServer(t *testing.T, h http.HandlerFunc) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(noulBody))
}

func TestNewClientMissingKeyFailsBeforeAnyRequest(t *testing.T) {
	srv, hits := newServer(t, okHandler)
	setEnv(t, "", srv.URL)

	c, err := NewClient()
	if err == nil {
		t.Fatalf("NewClient() = %v, want configuration error", c)
	}
	if !errors.Is(err, jev.ErrConfig) {
		t.Fatalf("NewClient() error %v does not wrap jev.ErrConfig", err)
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("server received %d requests, want 0", got)
	}
}

func TestNewClientSendsBearerTokenToConfiguredEndpoint(t *testing.T) {
	var auth, path string
	srv, _ := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		auth, path = r.Header.Get("Authorization"), r.URL.Path
		okHandler(w, r)
	})
	setEnv(t, "test-key", srv.URL)

	c, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}
	resp, err := c.SystemOne(context.Background(), safeRequest())
	if err != nil {
		t.Fatalf("SystemOne() error: %v", err)
	}
	if auth != "Bearer test-key" {
		t.Fatalf("Authorization = %q, want %q", auth, "Bearer test-key")
	}
	if path != "/v1/systemone" {
		t.Fatalf("path = %q, want /v1/systemone", path)
	}
	if a, ok := resp.Noul("safe"); !ok || a.Noul != 0.9 {
		t.Fatalf("Noul(safe) = %v, %v; want 0.9, true", a, ok)
	}
}

func TestNewClientDefaultsToTypeSafeEndpoint(t *testing.T) {
	setEnv(t, "test-key", "")

	c, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}
	if got, want := c.BaseURL(), "https://api.typesafe.ai"; got != want {
		t.Fatalf("BaseURL() = %q, want %q", got, want)
	}
}

func TestNewClientRefusesRemotePlainHTTP(t *testing.T) {
	setEnv(t, "test-key", "http://example.com")

	_, err := NewClient()
	if !errors.Is(err, jev.ErrConfig) {
		t.Fatalf("NewClient() error = %v, want jev.ErrConfig", err)
	}
}
