package jevclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/kataras/jev"
	"github.com/rshade/ax-go/contract"
)

func callAndClassify(t *testing.T, url string) *contract.Error {
	t.Helper()
	return callAndClassifyWithKey(t, "test-key", url)
}

func callAndClassifyWithKey(t *testing.T, key, url string) *contract.Error {
	t.Helper()
	setEnv(t, key, url)
	c, err := NewClient(fastBackoff)
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}
	_, err = c.SystemOne(context.Background(), safeRequest())
	if err == nil {
		t.Fatal("SystemOne() succeeded, want error")
	}
	var ce *contract.Error
	if !errors.As(Classify(context.Background(), err), &ce) {
		t.Fatalf("Classify(%v) is not a *contract.Error", err)
	}
	return ce
}

func TestClassifyMapsStatusToCode(t *testing.T) {
	tests := []struct {
		status        int
		wantCode      string
		wantExit      int
		wantRetryable *bool
	}{
		{http.StatusUnauthorized, CodeUnauthorized, contract.ExitAuth, ptr(false)},
		{http.StatusUnprocessableEntity, CodeInvalidRequest, contract.ExitValidation, ptr(false)},
		{http.StatusTooManyRequests, CodeRateLimited, contract.ExitNetwork, ptr(true)},
		{529, CodeOverloaded, contract.ExitNetwork, ptr(true)},
		{http.StatusTeapot, CodeStatus, contract.ExitInternal, nil},
		{http.StatusInternalServerError, CodeStatus, contract.ExitInternal, nil},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			url, _ := statusServer(t, tt.status)

			ce := callAndClassify(t, url)

			if ce.ErrorCode != tt.wantCode {
				t.Errorf("ErrorCode = %q, want %q", ce.ErrorCode, tt.wantCode)
			}
			if got := ce.ExitCode(); got != tt.wantExit {
				t.Errorf("ExitCode() = %d, want %d", got, tt.wantExit)
			}
			if got := ce.Context["status"]; got != tt.status {
				t.Errorf("Context[status] = %v, want %d", got, tt.status)
			}
			switch {
			case tt.wantRetryable == nil && ce.Retryable != nil:
				t.Errorf("Retryable = %v, want unspecified", *ce.Retryable)
			case tt.wantRetryable != nil && (ce.Retryable == nil || *ce.Retryable != *tt.wantRetryable):
				t.Errorf("Retryable = %v, want %v", ce.Retryable, *tt.wantRetryable)
			}
		})
	}
}

func bodyServer(t *testing.T, status int, contentType, body string) string {
	t.Helper()
	srv, _ := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
	return srv.URL
}

func TestClassifyPreservesServerDetail(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		body          string
		wantMessage   string
		wantErrorType string
	}{
		{
			name:          "object detail",
			status:        http.StatusTooManyRequests,
			body:          `{"detail":{"error_type":"rate_limit","message":"slow down"}}`,
			wantMessage:   "slow down",
			wantErrorType: "rate_limit",
		},
		{
			name:   "list detail names each field",
			status: http.StatusUnprocessableEntity,
			body: `{"detail":[{"loc":["body","state"],"msg":"Field required","type":"missing"},` +
				`{"loc":["body","questions",0],"msg":"Invalid","type":"x"}]}`,
			wantMessage: "state: Field required; questions.0: Invalid",
		},
		{
			name:        "object detail without error_type",
			status:      http.StatusUnauthorized,
			body:        `{"detail":{"message":"bad key"}}`,
			wantMessage: "bad key",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ce := callAndClassify(t, bodyServer(t, tt.status, "application/json", tt.body))

			if ce.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", ce.Message, tt.wantMessage)
			}
			got, present := ce.Context["error_type"]
			if tt.wantErrorType == "" && present {
				t.Errorf("Context[error_type] = %v, want absent", got)
			}
			if tt.wantErrorType != "" && got != tt.wantErrorType {
				t.Errorf("Context[error_type] = %v, want %q", got, tt.wantErrorType)
			}
		})
	}
}

func TestClassifyHandlesNonJSONBody(t *testing.T) {
	body := "<html>" + strings.Repeat("x", 500) + "</html>"

	ce := callAndClassify(t, bodyServer(t, http.StatusBadGateway, "text/html", body))

	if ce.ErrorCode != CodeStatus {
		t.Errorf("ErrorCode = %q, want %q", ce.ErrorCode, CodeStatus)
	}
	if !strings.HasSuffix(ce.Message, "...") || len(ce.Message) > 210 {
		t.Errorf("Message length %d, want a truncated message ending in ...", len(ce.Message))
	}
	if _, present := ce.Context["error_type"]; present {
		t.Errorf("Context[error_type] present for a non-JSON body")
	}
}

func TestClassifyNeverContainsTheAPIKey(t *testing.T) {
	const key = "sk-live-abc123XYZ"
	tests := []struct {
		name        string
		status      int
		contentType string
		body        string
	}{
		{
			name:        "object detail echoes key",
			status:      http.StatusUnauthorized,
			contentType: "application/json",
			body:        `{"detail":{"error_type":"bad-` + key + `","message":"rejected Bearer ` + key + `"}}`,
		},
		{
			name:        "list detail echoes key",
			status:      http.StatusUnprocessableEntity,
			contentType: "application/json",
			body:        `{"detail":[{"loc":["body","` + key + `"],"msg":"got ` + key + `","type":"x"}]}`,
		},
		{
			name:        "raw body echoes key",
			status:      http.StatusBadGateway,
			contentType: "text/plain",
			body:        "upstream saw Authorization: Bearer " + key,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ce := callAndClassifyWithKey(t, key, bodyServer(t, tt.status, tt.contentType, tt.body))

			encoded, err := json.Marshal(ce)
			if err != nil {
				t.Fatalf("json.Marshal() error: %v", err)
			}
			for name, text := range map[string]string{
				"envelope": string(encoded),
				"Error()":  ce.Error(),
			} {
				if strings.Contains(text, key) {
					t.Errorf("%s contains the API key: %s", name, text)
				}
			}
			if !strings.Contains(string(encoded), "[REDACTED]") {
				t.Errorf("envelope has no redaction marker: %s", encoded)
			}
			if cause := ce.Unwrap(); cause != nil && strings.Contains(cause.Error(), key) {
				t.Errorf("cause contains the API key: %v", cause)
			}
		})
	}
}

func TestClassifyInvalidSuccessfulResponse(t *testing.T) {
	srv, _ := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"model":"m","answers":{},"usage":{"input_tokens":1,"output_tokens":1}}`))
	})

	ce := callAndClassify(t, srv.URL)

	if ce.ErrorCode != CodeInvalidResponse {
		t.Errorf("ErrorCode = %q, want %q", ce.ErrorCode, CodeInvalidResponse)
	}
	if ce.Retryable != nil {
		t.Errorf("Retryable = %v, want unspecified", *ce.Retryable)
	}
}

func TestClassifyRequestRejectedBeforeSending(t *testing.T) {
	srv, hits := newServer(t, okHandler)
	setEnv(t, "test-key", srv.URL)
	c, err := NewClient(fastBackoff)
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}

	_, err = c.SystemOne(context.Background(), jev.Request{State: "x"})
	var ce *contract.Error
	if !errors.As(Classify(context.Background(), err), &ce) {
		t.Fatalf("Classify(%v) is not a *contract.Error", err)
	}

	if ce.ErrorCode != CodeInvalidRequest {
		t.Errorf("ErrorCode = %q, want %q", ce.ErrorCode, CodeInvalidRequest)
	}
	if got := ce.ExitCode(); got != contract.ExitValidation {
		t.Errorf("ExitCode() = %d, want %d", got, contract.ExitValidation)
	}
	if got := hits.Load(); got != 0 {
		t.Errorf("server hits = %d, want 0 (no spend on an invalid request)", got)
	}
}
