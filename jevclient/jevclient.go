// Package jevclient builds the Jev client used by jev-decide and classifies
// its failures.
//
// [NewClient] is the supported constructor. It returns a [jev.Client] with
// these policies applied:
//
//   - The bearer token comes only from the TYPESAFE_API_KEY environment
//     variable. Construction fails with [jev.ErrConfig] when it is empty.
//   - Requests go to https://api.typesafe.ai unless TYPESAFE_BASE_URL is set.
//     A base URL must be https, or http to a loopback IP address (a name such
//     as localhost is refused, since it can be pointed elsewhere), and must not carry
//     credentials, a query or a fragment. Redirects are never followed, so the
//     token cannot be sent to another host.
//   - Only 429 and 529 are retried, at most twice, within a 30 second budget,
//     honoring Retry-After up to 10 seconds. Timeouts and
//     connection failures are not retried, because the server may already have
//     processed the request.
//   - A successful response that omits a requested answer, answers with a
//     different type, or reports a probability outside 0 to 1 is an error, never
//     a zero value.
//
// [Classify] turns any failure from a call into a structured error.
package jevclient

import (
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kataras/jev"
)

// Option adjusts how [NewClient] builds the client.
type Option func(*config)

type config struct {
	retry     jev.RetryPolicy
	timeout   time.Duration
	logOutput io.Writer
}

// WithTimeout sets the per-attempt timeout. Zero keeps the client default.
func WithTimeout(d time.Duration) Option {
	return func(c *config) { c.timeout = d }
}

const (
	statusOverloaded = 529
	maxRetries       = 2
	maxRetryAfter    = 10 * time.Second
	maxElapsed       = 30 * time.Second
)

// defaultRetryPolicy retries only statuses that mean the request was not
// processed, so a retry cannot bill twice. Timeouts and connection failures
// are not retried because the server may already have processed the request.
func defaultRetryPolicy() jev.RetryPolicy {
	p := jev.DefaultRetryPolicy()
	p.MaxRetries = maxRetries
	p.Statuses = []int{http.StatusTooManyRequests, statusOverloaded}
	p.RetryConnection = false
	p.RetryTimeout = false
	p.MaxRetryAfter = maxRetryAfter
	p.MaxElapsed = maxElapsed
	return p
}

// NewClient returns a Jev client configured from the environment.
func NewClient(opts ...Option) (*jev.Client, error) {
	cfg := config{retry: defaultRetryPolicy()}
	for _, opt := range opts {
		opt(&cfg)
	}
	baseURL, err := validateBaseURL(strings.TrimSpace(os.Getenv("TYPESAFE_BASE_URL")))
	if err != nil {
		return nil, err
	}
	logger, err := newLogger(cfg.logOutput, strings.TrimSpace(os.Getenv("TYPESAFE_API_KEY")))
	if err != nil {
		return nil, err
	}
	jevOpts := []jev.Option{jev.WithBaseURL(baseURL), jev.WithRetry(cfg.retry), jev.WithLogger(logger)}
	if cfg.timeout > 0 {
		jevOpts = append(jevOpts, jev.WithTimeout(cfg.timeout))
	}
	return jev.New(jevOpts...)
}
