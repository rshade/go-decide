// Package jevclient builds the Jev client used by go-decide and classifies
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
//   - Logging follows TYPESAFE_LOG_LEVEL: debug, info, warn, error or off.
//     Unset or off writes nothing. Otherwise logs go to stderr, and the API
//     key is replaced in every attribute. An invalid level makes NewClient
//     return jev.ErrConfig.
//   - A successful response that omits a requested answer, answers with a
//     different type, or reports a probability outside 0 to 1 is an error, never
//     a zero value.
//
// [Classify] turns any failure from a call into a structured error.
//
// # Response cache
//
// [WithResponseCache] is opt-in; without it nothing is written to disk. With
// it, a successful response is stored in the named directory under the
// SHA-256 of the request's method, scheme, host, path and body, and the same
// request is answered from there without a network call. Headers are not part
// of the key or the entry, so the token is never written. Only a 200 that
// answers every question asked is stored; errors, retried statuses, redirects
// and a response missing an answer are not.
// The directory is created readable only by its owner. Entries are never
// invalidated: a changed model behind the same request still gets the old
// answer, so delete the directory to measure again.
package jevclient

import (
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kataras/jev"

	"github.com/rshade/go-decide/internal/clientkit"
)

// Option adjusts how [NewClient] builds the client.
type Option func(*config)

type config struct {
	retry     jev.RetryPolicy
	timeout   time.Duration
	logOutput io.Writer
	cacheDir  string
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

// WithResponseCache stores each successful response in dir and answers a
// repeated request from there without a network call. See the package doc.
func WithResponseCache(dir string) Option {
	return func(c *config) { c.cacheDir = dir }
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
	logger, err := clientkit.NewLogger(cfg.logOutput, "TYPESAFE_LOG_LEVEL", strings.TrimSpace(os.Getenv("TYPESAFE_API_KEY")))
	if err != nil {
		return nil, err
	}
	jevOpts := []jev.Option{jev.WithBaseURL(baseURL), jev.WithRetry(cfg.retry), jev.WithLogger(logger)}
	if cfg.timeout > 0 {
		jevOpts = append(jevOpts, jev.WithTimeout(cfg.timeout))
	}
	if cfg.cacheDir != "" {
		jevOpts = append(jevOpts, jev.WithHTTPClient(clientkit.NewCachingClient(cfg.cacheDir, http.DefaultTransport)))
	}
	return jev.New(jevOpts...)
}
