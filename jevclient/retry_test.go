package jevclient

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kataras/jev"
)

func fastBackoff(c *config) {
	c.retry.InitialBackoff = time.Millisecond
	c.retry.MaxBackoff = 5 * time.Millisecond
}

func statusServer(t *testing.T, statuses ...int) (url string, hits *atomic.Int32) {
	t.Helper()
	var n atomic.Int32
	srv, _ := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		i := int(n.Add(1)) - 1
		status := statuses[min(i, len(statuses)-1)]
		if status == http.StatusOK {
			okHandler(w, r)
			return
		}
		w.Header().Set("Retry-After-Ms", "5")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"detail":{"error_type":"x","message":"m"}}`))
	})
	return srv.URL, &n
}

func TestRetryPolicyIsExplicitAndBounded(t *testing.T) {
	p := defaultRetryPolicy()

	if want := []int{http.StatusTooManyRequests, 529}; !slices.Equal(p.Statuses, want) {
		t.Errorf("Statuses = %v, want %v", p.Statuses, want)
	}
	if p.RetryConnection || p.RetryTimeout {
		t.Errorf("RetryConnection=%v RetryTimeout=%v, want both false", p.RetryConnection, p.RetryTimeout)
	}
	if !p.RespectRetryAfter || p.MaxRetryAfter <= 0 || p.MaxRetryAfter > p.MaxElapsed {
		t.Errorf("RespectRetryAfter=%v MaxRetryAfter=%v MaxElapsed=%v, want honored with a ceiling inside the budget",
			p.RespectRetryAfter, p.MaxRetryAfter, p.MaxElapsed)
	}
	if p.MaxRetries != 2 {
		t.Errorf("MaxRetries = %d, want 2", p.MaxRetries)
	}
	if p.MaxElapsed <= 0 {
		t.Errorf("MaxElapsed = %v, want a total time budget", p.MaxElapsed)
	}
}

func TestRateLimitedCallIsRetriedThenSucceeds(t *testing.T) {
	url, hits := statusServer(t, http.StatusTooManyRequests, http.StatusOK)
	setEnv(t, "test-key", url)
	c, err := NewClient(fastBackoff)
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}

	resp, err := c.SystemOne(context.Background(), safeRequest())
	if err != nil {
		t.Fatalf("SystemOne() error: %v", err)
	}
	if got := hits.Load(); got != 2 {
		t.Fatalf("server hits = %d, want 2", got)
	}
	if resp.Attempts != 2 {
		t.Fatalf("Attempts = %d, want 2", resp.Attempts)
	}
}

func TestOverloadedCallStopsAtRetryLimit(t *testing.T) {
	url, hits := statusServer(t, 529)
	setEnv(t, "test-key", url)
	c, err := NewClient(fastBackoff)
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}

	_, err = c.SystemOne(context.Background(), safeRequest())
	if !errors.Is(err, jev.ErrOverloaded) {
		t.Fatalf("SystemOne() error = %v, want jev.ErrOverloaded", err)
	}
	if got := hits.Load(); got != 3 {
		t.Fatalf("server hits = %d, want 3 (1 attempt + 2 retries)", got)
	}
}

func TestOtherStatusesAreNotRetried(t *testing.T) {
	for _, status := range []int{
		http.StatusUnauthorized,
		http.StatusUnprocessableEntity,
		http.StatusRequestTimeout,
		http.StatusInternalServerError,
		http.StatusBadGateway,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			url, hits := statusServer(t, status)
			setEnv(t, "test-key", url)
			c, err := NewClient(fastBackoff)
			if err != nil {
				t.Fatalf("NewClient() error: %v", err)
			}

			if _, err = c.SystemOne(context.Background(), safeRequest()); err == nil {
				t.Fatal("SystemOne() succeeded, want error")
			}
			if got := hits.Load(); got != 1 {
				t.Fatalf("server hits = %d, want 1", got)
			}
		})
	}
}

func TestRetryAfterAboveCeilingFallsBackToBackoff(t *testing.T) {
	var n atomic.Int32
	srv, _ := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) == 1 {
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		okHandler(w, r)
	})
	setEnv(t, "test-key", srv.URL)
	c, err := NewClient(fastBackoff)
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}

	start := time.Now()
	if _, err = c.SystemOne(context.Background(), safeRequest()); err != nil {
		t.Fatalf("SystemOne() error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("call took %v, a Retry-After above the ceiling must not be honored", elapsed)
	}
}

func TestRetryBudgetStopsFurtherRetries(t *testing.T) {
	var n atomic.Int32
	srv, _ := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		n.Add(1)
		w.Header().Set("Retry-After-Ms", "200")
		w.WriteHeader(statusOverloaded)
	})
	setEnv(t, "test-key", srv.URL)
	c, err := NewClient(func(c *config) { c.retry.MaxElapsed = 250 * time.Millisecond })
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}

	_, err = c.SystemOne(context.Background(), safeRequest())
	if !errors.Is(err, jev.ErrOverloaded) {
		t.Fatalf("SystemOne() error = %v, want jev.ErrOverloaded", err)
	}
	if got := n.Load(); got != 2 {
		t.Fatalf("server hits = %d, want 2: a third attempt would exceed the 250ms budget", got)
	}
}
