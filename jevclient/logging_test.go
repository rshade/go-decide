package jevclient

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/kataras/jev"
)

func logsTo(buf *bytes.Buffer) Option {
	return func(c *config) { c.logOutput = buf }
}

func echoKeyServer(t *testing.T, key string) string {
	t.Helper()
	return bodyServer(t, http.StatusUnauthorized, "application/json",
		`{"detail":{"error_type":"bad_key","message":"rejected Bearer `+key+`"}}`)
}

func TestLogsNeverContainTheAPIKey(t *testing.T) {
	const key = "sk-live-abc123XYZ"
	url := echoKeyServer(t, key)
	setEnv(t, key, url)
	t.Setenv("TYPESAFE_LOG_LEVEL", "debug")
	var logs bytes.Buffer
	c, err := NewClient(logsTo(&logs), fastBackoff)
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}

	_, _ = c.SystemOne(context.Background(), safeRequest())

	out := logs.String()
	if !strings.Contains(out, "typesafe response body") {
		t.Fatalf("debug logs missing the response body line, got: %q", out)
	}
	if strings.Contains(out, key) {
		t.Errorf("logs contain the API key: %s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Errorf("logs have no redaction marker: %s", out)
	}
}

func TestNoLogsUnlessLevelIsSet(t *testing.T) {
	url, _ := statusServer(t, http.StatusOK)
	setEnv(t, "test-key", url)
	var logs bytes.Buffer
	c, err := NewClient(logsTo(&logs))
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}

	if _, err = c.SystemOne(context.Background(), safeRequest()); err != nil {
		t.Fatalf("SystemOne() error: %v", err)
	}

	if logs.Len() != 0 {
		t.Fatalf("logged %q with TYPESAFE_LOG_LEVEL unset, want no output", logs.String())
	}
}

func TestInvalidLogLevelIsAConfigurationError(t *testing.T) {
	setEnv(t, "test-key", "")
	t.Setenv("TYPESAFE_LOG_LEVEL", "verbose")

	_, err := NewClient()

	if !errors.Is(err, jev.ErrConfig) {
		t.Fatalf("NewClient() error = %v, want jev.ErrConfig", err)
	}
}

func TestRetryLogRedactsKeyOn429(t *testing.T) {
	assertRetryLogRedacts(t, http.StatusTooManyRequests)
}

func TestRetryLogRedactsKeyOn5xx(t *testing.T) {
	// 529 is the 5xx this client retries, so the dependency logs typesafe retry.
	assertRetryLogRedacts(t, statusOverloaded)
}

func TestRetryLogRedactsKeyWithResponseCache(t *testing.T) {
	assertRetryLogRedacts(t, http.StatusTooManyRequests, WithResponseCache(t.TempDir()))
}

func assertRetryLogRedacts(t *testing.T, status int, extra ...Option) {
	t.Helper()
	const key = "sk-live-abc123XYZ"
	url := bodyServer(t, status, "application/json",
		`{"detail":{"error_type":"echo","message":"rejected Bearer `+key+`"}}`)
	setEnv(t, key, url)
	t.Setenv("TYPESAFE_LOG_LEVEL", "info")

	var logs bytes.Buffer
	c, err := NewClient(append([]Option{logsTo(&logs), fastBackoff}, extra...)...)
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}
	_, _ = c.SystemOne(context.Background(), safeRequest())

	out := logs.String()
	if !strings.Contains(out, "typesafe retry") {
		t.Fatalf("info logs missing the typesafe retry line, got: %q", out)
	}
	if strings.Contains(out, key) {
		t.Errorf("logs contain the API key: %s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Errorf("logs have no redaction marker: %s", out)
	}
}
