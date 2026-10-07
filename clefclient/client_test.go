package clefclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kataras/jev"
)

const (
	testAccount = "0123456789abcdef0123456789abcdef"
	testToken   = "cf-secret-token"
	okEnvelope  = `{"result":{"model":"clef","answers":{"urgent":{"type":"noul","noul":0.9}},"usage":{"input_tokens":3,"output_tokens":0}},"success":true,"errors":[],"messages":[]}`
)

func setEnv(t *testing.T, baseURL string) {
	t.Helper()
	t.Setenv("CLOUDFLARE_AUTH_TOKEN", testToken)
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", testAccount)
	t.Setenv("CLOUDFLARE_BASE_URL", baseURL)
	t.Setenv("CLOUDFLARE_LOG_LEVEL", "")
}

func urgentRequest() jev.Request {
	return jev.Request{
		State:     "Checkout is failing for every customer.",
		Questions: jev.Questions{"urgent": jev.Noul{Instructions: "Is this urgent?"}},
	}
}

func TestNewClientRejectsBadConfigurationBeforeAnyRequest(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer srv.Close()

	tests := []struct {
		name    string
		mutate  func(t *testing.T)
		wantVar string
	}{
		{"token missing", func(t *testing.T) { t.Setenv("CLOUDFLARE_AUTH_TOKEN", "") }, "CLOUDFLARE_AUTH_TOKEN"},
		{"account missing", func(t *testing.T) { t.Setenv("CLOUDFLARE_ACCOUNT_ID", "") }, "CLOUDFLARE_ACCOUNT_ID"},
		{"account with slash", func(t *testing.T) { t.Setenv("CLOUDFLARE_ACCOUNT_ID", "../"+testAccount[3:]) }, "CLOUDFLARE_ACCOUNT_ID"},
		{"account with query", func(t *testing.T) { t.Setenv("CLOUDFLARE_ACCOUNT_ID", testAccount[:30]+"?x") }, "CLOUDFLARE_ACCOUNT_ID"},
		{"remote http", func(t *testing.T) { t.Setenv("CLOUDFLARE_BASE_URL", "http://example.com") }, "base URL"},
		{"loopback name", func(t *testing.T) { t.Setenv("CLOUDFLARE_BASE_URL", "http://localhost:8080") }, "base URL"},
		{"bad log level", func(t *testing.T) { t.Setenv("CLOUDFLARE_LOG_LEVEL", "loud") }, "CLOUDFLARE_LOG_LEVEL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, srv.URL)
			tt.mutate(t)
			_, err := NewClient()
			if !errors.Is(err, jev.ErrConfig) {
				t.Fatalf("err = %v, want one wrapping jev.ErrConfig", err)
			}
			if !strings.Contains(err.Error(), tt.wantVar) {
				t.Errorf("err = %q, want it to name %q", err, tt.wantVar)
			}
			if strings.Contains(err.Error(), testToken) {
				t.Errorf("err = %q leaks the token", err)
			}
		})
	}
	if hits.Load() != 0 {
		t.Errorf("%d requests were sent during failed construction", hits.Load())
	}
}

func TestRequestShape(t *testing.T) {
	var gotPath, gotAuth, gotMethod string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth, gotMethod = r.URL.Path, r.Header.Get("Authorization"), r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(okEnvelope))
	}))
	defer srv.Close()
	setEnv(t, srv.URL)

	c, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.SystemOne(context.Background(), urgentRequest())
	if err != nil {
		t.Fatal(err)
	}

	if want := "/client/v4/accounts/" + testAccount + "/ai/run/@cf/cloudflare/clef"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
	if gotMethod != http.MethodPost || gotAuth != "Bearer "+testToken {
		t.Errorf("method %q auth %q", gotMethod, gotAuth)
	}
	if gotBody["model"] != "clef" || gotBody["state"] == nil || gotBody["questions"] == nil {
		t.Errorf("body %v lacks model, state or questions", gotBody)
	}
	if resp.Usage.InputTokens != 3 {
		t.Errorf("usage = %+v, want the result's usage", resp.Usage)
	}
}

func TestRedirectIsNotFollowed(t *testing.T) {
	var hits atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusFound)
	}))
	defer srv.Close()
	setEnv(t, srv.URL)

	c, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.SystemOne(context.Background(), urgentRequest()); err == nil {
		t.Fatal("expected an error for a redirect")
	}
	if hits.Load() != 0 {
		t.Errorf("redirect target received %d requests", hits.Load())
	}
}

func TestEnvelopeFailuresAreErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"success false", `{"result":{},"success":false,"errors":[{"code":7000,"message":"boom"}],"messages":[]}`},
		{"no result", `{"success":true,"errors":[],"messages":[]}`},
		{"missing answer", `{"result":{"model":"clef","answers":{},"usage":{"input_tokens":1}},"success":true}`},
		{"wrong type", `{"result":{"answers":{"urgent":{"type":"score","score":1.0}},"usage":{}},"success":true}`},
		{"probability out of range", `{"result":{"answers":{"urgent":{"type":"noul","noul":1.5}},"usage":{}},"success":true}`},
		{"not json", `<html>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()
			setEnv(t, srv.URL)
			c, err := NewClient()
			if err != nil {
				t.Fatal(err)
			}
			resp, err := c.SystemOne(context.Background(), urgentRequest())
			if err == nil {
				t.Fatalf("expected an error, got answers %v", resp)
			}
		})
	}
}

func TestOnly429IsRetried(t *testing.T) {
	tests := []struct {
		name     string
		statuses []int
		wantHits int32
		wantOK   bool
	}{
		{"429 then 200", []int{429, 200}, 2, true},
		{"429 exhausts after two retries", []int{429, 429, 429, 429}, 3, false},
		{"500 is not retried", []int{500, 200}, 1, false},
		{"529 is not retried", []int{529, 200}, 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hits atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				i := int(hits.Add(1)) - 1
				status := tt.statuses[min(i, len(tt.statuses)-1)]
				if status != 200 {
					w.Header().Set("Retry-After", "0")
					w.WriteHeader(status)
					return
				}
				_, _ = w.Write([]byte(okEnvelope))
			}))
			defer srv.Close()
			setEnv(t, srv.URL)
			c, err := NewClient()
			if err != nil {
				t.Fatal(err)
			}
			_, err = c.SystemOne(context.Background(), urgentRequest())
			if (err == nil) != tt.wantOK {
				t.Errorf("err = %v, wantOK %v", err, tt.wantOK)
			}
			if hits.Load() != tt.wantHits {
				t.Errorf("hits = %d, want %d", hits.Load(), tt.wantHits)
			}
		})
	}
}

func TestTimeoutIsNotRetried(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer srv.Close()
	setEnv(t, srv.URL)
	c, err := NewClient(WithTimeout(50 * time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.SystemOne(context.Background(), urgentRequest())
	if !errors.Is(err, jev.ErrTimeout) {
		t.Fatalf("err = %v, want a timeout", err)
	}
	if hits.Load() != 1 {
		t.Errorf("hits = %d, want 1", hits.Load())
	}
}

func TestTokenNeverAppearsInLogs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors":[{"code":5006,"message":"echo ` + testToken + `"}],"success":false}`))
	}))
	defer srv.Close()
	setEnv(t, srv.URL)
	t.Setenv("CLOUDFLARE_LOG_LEVEL", "debug")

	var logs bytes.Buffer
	c, err := NewClient(func(cfg *config) { cfg.logOutput = &logs })
	if err != nil {
		t.Fatal(err)
	}
	_, _ = c.SystemOne(context.Background(), urgentRequest())
	if logs.Len() == 0 {
		t.Fatal("expected debug logs")
	}
	if strings.Contains(logs.String(), testToken) {
		t.Errorf("logs leak the token:\n%s", logs.String())
	}
}

func TestResponseCacheAnswersRepeatsWithoutARequest(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(okEnvelope))
	}))
	defer srv.Close()
	setEnv(t, srv.URL)
	c, err := NewClient(WithResponseCache(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := c.SystemOne(context.Background(), urgentRequest()); err != nil {
			t.Fatal(err)
		}
	}
	if hits.Load() != 1 {
		t.Errorf("hits = %d, want 1", hits.Load())
	}
}
