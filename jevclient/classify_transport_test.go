package jevclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/kataras/jev"
	"github.com/rshade/ax-go/contract"
)

func TestClassifyReportsRequestID(t *testing.T) {
	srv, _ := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Typesafe-Request-Id", "req_123")
		w.WriteHeader(http.StatusUnauthorized)
	})

	ce := callAndClassify(t, srv.URL)

	if got := ce.Context["request_id"]; got != "req_123" {
		t.Fatalf("Context[request_id] = %v, want req_123", got)
	}
}

func TestClassifyOmitsRequestIDWhenServerSendsNone(t *testing.T) {
	url, _ := statusServer(t, http.StatusUnauthorized)

	ce := callAndClassify(t, url)

	if got, present := ce.Context["request_id"]; present {
		t.Fatalf("Context[request_id] = %v, want absent", got)
	}
}

func TestClassifyReportsRetryAfterInWholeSecondsRoundedUp(t *testing.T) {
	tests := []struct {
		name   string
		status int
		header http.Header
		want   int64
	}{
		{"seconds", 429, http.Header{"Retry-After": {"3"}}, 3},
		{"milliseconds round up", 429, http.Header{"Retry-After-Ms": {"1500"}}, 2},
		{"one millisecond rounds up", 529, http.Header{"Retry-After-Ms": {"1"}}, 1},
		{"absent", 529, http.Header{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiErr := &jev.APIError{Status: tt.status, Header: tt.header}

			var ce *contract.Error
			if !errors.As(Classify(context.Background(), apiErr), &ce) {
				t.Fatal("Classify() is not a *contract.Error")
			}

			if ce.Retryable == nil || !*ce.Retryable {
				t.Errorf("Retryable = %v, want true", ce.Retryable)
			}
			if ce.RetryAfterSeconds != tt.want {
				t.Errorf("RetryAfterSeconds = %d, want %d", ce.RetryAfterSeconds, tt.want)
			}
		})
	}
}

func TestClassifyOmitsRetryAfterWhenNotRetryable(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			apiErr := &jev.APIError{Status: status, Header: http.Header{"Retry-After": {"5"}}}

			var ce *contract.Error
			if !errors.As(Classify(context.Background(), apiErr), &ce) {
				t.Fatal("Classify() is not a *contract.Error")
			}

			if ce.RetryAfterSeconds != 0 {
				t.Errorf("RetryAfterSeconds = %d, want 0 for a status that is not retryable", ce.RetryAfterSeconds)
			}
		})
	}
}

func slowServer(t *testing.T) (url string, hits func() int32) {
	t.Helper()
	srv, n := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	})
	return srv.URL, n.Load
}

func TestClassifyTimeoutIsDistinctAndNotRetried(t *testing.T) {
	url, hits := slowServer(t)
	setEnv(t, "test-key", url)
	c, err := NewClient(fastBackoff, WithTimeout(30*time.Millisecond))
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}

	_, err = c.SystemOne(context.Background(), safeRequest())
	var ce *contract.Error
	if !errors.As(Classify(context.Background(), err), &ce) {
		t.Fatalf("Classify(%v) is not a *contract.Error", err)
	}

	if ce.ErrorCode != CodeTimeout {
		t.Errorf("ErrorCode = %q, want %q", ce.ErrorCode, CodeTimeout)
	}
	if ce.Retryable == nil || !*ce.Retryable {
		t.Errorf("Retryable = %v, want true", ce.Retryable)
	}
	if got := ce.ExitCode(); got != contract.ExitNetwork {
		t.Errorf("ExitCode() = %d, want %d", got, contract.ExitNetwork)
	}
	if got := hits(); got != 1 {
		t.Errorf("server hits = %d, want 1 (timeouts are not retried)", got)
	}
}

func TestClassifyConnectionFailureIsDistinct(t *testing.T) {
	srv, _ := newServer(t, okHandler)
	setEnv(t, "test-key", srv.URL)
	c, err := NewClient(fastBackoff)
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}
	srv.Close()

	_, err = c.SystemOne(context.Background(), safeRequest())
	var ce *contract.Error
	if !errors.As(Classify(context.Background(), err), &ce) {
		t.Fatalf("Classify(%v) is not a *contract.Error", err)
	}

	if ce.ErrorCode != CodeConnection {
		t.Errorf("ErrorCode = %q, want %q", ce.ErrorCode, CodeConnection)
	}
	if ce.Retryable == nil || !*ce.Retryable {
		t.Errorf("Retryable = %v, want true", ce.Retryable)
	}
}

func TestClassifyPassesCallerCancellationThrough(t *testing.T) {
	tests := []struct {
		name string
		ctx  func() (context.Context, context.CancelFunc)
		want error
	}{
		{"canceled", func() (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx, cancel
		}, context.Canceled},
		{"deadline", func() (context.Context, context.CancelFunc) {
			return context.WithTimeout(context.Background(), 30*time.Millisecond)
		}, context.DeadlineExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, _ := slowServer(t)
			setEnv(t, "test-key", url)
			c, err := NewClient(fastBackoff)
			if err != nil {
				t.Fatalf("NewClient() error: %v", err)
			}
			ctx, cancel := tt.ctx()
			defer cancel()

			_, err = c.SystemOne(ctx, safeRequest())
			got := Classify(ctx, err)

			if !errors.Is(got, tt.want) {
				t.Fatalf("Classify() = %v, want it to be %v", got, tt.want)
			}
			var ce *contract.Error
			if errors.As(got, &ce) {
				t.Fatalf("Classify() wrapped cancellation as jev error %q", ce.ErrorCode)
			}
		})
	}
}
