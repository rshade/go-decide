package clefclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rshade/ax-go/contract"
)

func classifyServer(t *testing.T, status int, body string, header http.Header) error {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for k, v := range header {
			w.Header()[k] = v
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	setEnv(t, srv.URL)
	c, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.SystemOne(context.Background(), urgentRequest())
	if err == nil {
		t.Fatal("expected a failure")
	}
	return Classify(context.Background(), err)
}

func TestClassifyHTTPFailures(t *testing.T) {
	authBody := `{"result":null,"success":false,"errors":[{"code":10000,"message":"Authentication error"}],"messages":[]}`
	badBody := `{"errors":[{"message":"AiError: Bad input","code":5006}],"success":false,"result":{},"messages":[]}`
	tests := []struct {
		name       string
		status     int
		body       string
		header     http.Header
		wantCode   string
		wantExit   int
		wantRetry  *bool
		wantCFCode int
	}{
		{"401", 401, authBody, nil, CodeUnauthorized, contract.ExitAuth, ptr(false), 10000},
		{"403", 403, authBody, nil, CodeUnauthorized, contract.ExitAuth, ptr(false), 10000},
		{"400 with request id", 400, badBody, http.Header{"Cf-Ai-Req-Id": {"abc-123"}}, CodeInvalidRequest, contract.ExitValidation, ptr(false), 5006},
		{"422", 422, badBody, nil, CodeInvalidRequest, contract.ExitValidation, ptr(false), 5006},
		{"success false inside a 200", 200, `{"result":{},"success":false,"errors":[{"code":7000,"message":"boom"}]}`, nil, CodeStatus, contract.ExitInternal, nil, 7000},
		{"418", 418, `teapot`, nil, CodeStatus, contract.ExitInternal, nil, 0},
		{"500", 500, `{"success":false,"errors":[{"code":3000,"message":"oops"}]}`, nil, CodeStatus, contract.ExitInternal, nil, 3000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := classifyServer(t, tt.status, tt.body, tt.header)
			var ce *contract.Error
			if !errors.As(err, &ce) {
				t.Fatalf("err = %T %v, want *contract.Error", err, err)
			}
			if ce.ErrorCode != tt.wantCode || ce.ExitCode() != tt.wantExit {
				t.Errorf("code %q exit %d, want %q %d", ce.ErrorCode, ce.ExitCode(), tt.wantCode, tt.wantExit)
			}
			if (ce.Retryable == nil) != (tt.wantRetry == nil) || (tt.wantRetry != nil && *ce.Retryable != *tt.wantRetry) {
				t.Errorf("retryable = %v, want %v", ce.Retryable, tt.wantRetry)
			}
			if tt.wantCFCode != 0 && ce.Context["cloudflare_code"] != tt.wantCFCode {
				t.Errorf("cloudflare_code = %v, want %d", ce.Context["cloudflare_code"], tt.wantCFCode)
			}
			if tt.header != nil && ce.Context["request_id"] != "abc-123" {
				t.Errorf("request_id = %v, want abc-123", ce.Context["request_id"])
			}
		})
	}
}

func TestClassifyRateLimitedIsRetryableWithRetryAfter(t *testing.T) {
	err := classifyServer(t, 429, `{"success":false,"errors":[{"code":2001,"message":"slow down"}]}`, http.Header{"Retry-After": {"60"}})
	var ce *contract.Error
	if !errors.As(err, &ce) {
		t.Fatal(err)
	}
	if ce.ErrorCode != CodeRateLimited || ce.Retryable == nil || !*ce.Retryable || ce.RetryAfterSeconds != 60 {
		t.Errorf("got %+v", ce)
	}
}

func TestClassifyRedactsTheTokenFromMessageAndContext(t *testing.T) {
	err := classifyServer(t, 400, `{"success":false,"errors":[{"code":5006,"message":"echo `+testToken+`"}]}`, nil)
	var ce *contract.Error
	if !errors.As(err, &ce) {
		t.Fatal(err)
	}
	if strings.Contains(ce.Message, testToken) || !strings.Contains(ce.Message, "[REDACTED]") {
		t.Errorf("message = %q", ce.Message)
	}
}

func TestClassifyTransportAndResponseFailures(t *testing.T) {
	t.Run("connection refused", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		setEnv(t, srv.URL)
		srv.Close()
		c, err := NewClient()
		if err != nil {
			t.Fatal(err)
		}
		_, err = c.SystemOne(context.Background(), urgentRequest())
		var ce *contract.Error
		if !errors.As(Classify(context.Background(), err), &ce) || ce.ErrorCode != CodeConnection || ce.ExitCode() != contract.ExitNetwork {
			t.Errorf("got %v", Classify(context.Background(), err))
		}
	})
	t.Run("missing answer", func(t *testing.T) {
		err := classifyServer200(t, `{"result":{"answers":{},"usage":{}},"success":true}`)
		var ce *contract.Error
		if !errors.As(err, &ce) || ce.ErrorCode != CodeInvalidResponse {
			t.Errorf("got %v", err)
		}
	})
	t.Run("caller cancellation passes through", func(t *testing.T) {
		if err := Classify(context.Background(), context.Canceled); !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("nil", func(t *testing.T) {
		if Classify(context.Background(), nil) != nil {
			t.Error("want nil")
		}
	})
}

func classifyServer200(t *testing.T, body string) error {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
	defer srv.Close()
	setEnv(t, srv.URL)
	c, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.SystemOne(context.Background(), urgentRequest())
	return Classify(context.Background(), err)
}
