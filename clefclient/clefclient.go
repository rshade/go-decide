// Package clefclient builds a client for Cloudflare's clef model and
// classifies its failures.
//
// [NewClient] returns a [jev.Client] that sends System One questions to
// Cloudflare Workers AI. Cloudflare takes the same questions and returns the
// same answers as System One, so the result plugs in wherever a Jev client
// does. These policies are applied:
//
//   - The bearer token comes only from CLOUDFLARE_AUTH_TOKEN and the account
//     from CLOUDFLARE_ACCOUNT_ID, which must be 32 hexadecimal characters.
//     Construction fails with [jev.ErrConfig] when either is missing or
//     malformed.
//   - Requests go to https://api.cloudflare.com unless CLOUDFLARE_BASE_URL is
//     set. A base URL must be https, or http to a loopback IP address, and
//     must not carry credentials, a query or a fragment. Redirects are never
//     followed, so the token cannot be sent to another host.
//   - Every request names the model clef.
//   - A response is a success only when the status is 200 and the envelope
//     reports success. A failure inside a 200 becomes an error, and an answer
//     that is missing, of the wrong type or outside 0 to 1 is an error, never a
//     zero value.
//   - Only 429 is retried, at most twice, within a 30 second budget, honoring
//     Retry-After up to 10 seconds. Timeouts and connection failures are not
//     retried, because the request may already have been billed.
//   - Logging follows CLOUDFLARE_LOG_LEVEL: debug, info, warn, error or off.
//     The token is replaced in every logged string.
//
// [Classify] turns any failure from a call into a structured error.
package clefclient

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/kataras/jev"

	"github.com/rshade/go-decide/internal/clientkit"
)

const (
	defaultBaseURL = "https://api.cloudflare.com"
	runPath        = "/client/v4/accounts/%s/ai/run/@cf/cloudflare"
	modelName      = "clef"

	maxRetries    = 2
	maxRetryAfter = 10 * time.Second
	maxElapsed    = 30 * time.Second
)

var accountIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)

// Option adjusts how [NewClient] builds the client.
type Option func(*config)

type config struct {
	timeout   time.Duration
	logOutput io.Writer
	cacheDir  string
}

// WithTimeout sets the per-attempt timeout. Zero keeps the client default.
func WithTimeout(d time.Duration) Option {
	return func(c *config) { c.timeout = d }
}

// WithResponseCache stores each successful response in dir and answers a
// repeated request from there without a network call. The key covers the
// method, host, path (which holds the account) and body, never a header.
func WithResponseCache(dir string) Option {
	return func(c *config) { c.cacheDir = dir }
}

func defaultRetryPolicy() jev.RetryPolicy {
	p := jev.DefaultRetryPolicy()
	p.MaxRetries = maxRetries
	p.Statuses = []int{http.StatusTooManyRequests}
	p.RetryConnection = false
	p.RetryTimeout = false
	p.MaxRetryAfter = maxRetryAfter
	p.MaxElapsed = maxElapsed
	return p
}

func accountPath(account string) string {
	return fmt.Sprintf(runPath, strings.ToLower(account))
}

// BaseURL returns the base URL that, with [NewTransport], makes a System One
// client talk to clef: https://api.cloudflare.com plus the account's run path.
// It is for callers that bring their own client; [NewClient] builds the same
// thing for a [jev.Client]. The account must be 32 hexadecimal characters.
func BaseURL(account string) (string, error) {
	if !accountIDPattern.MatchString(account) {
		return "", fmt.Errorf("%w: the Cloudflare account ID must be 32 hexadecimal characters", jev.ErrConfig)
	}
	return defaultBaseURL + accountPath(account), nil
}

// NewClient returns a clef client configured from the environment.
func NewClient(opts ...Option) (*jev.Client, error) {
	var cfg config
	for _, opt := range opts {
		opt(&cfg)
	}
	token := strings.TrimSpace(os.Getenv("CLOUDFLARE_AUTH_TOKEN"))
	if token == "" {
		return nil, fmt.Errorf("%w: CLOUDFLARE_AUTH_TOKEN is not set", jev.ErrConfig)
	}
	account := strings.TrimSpace(os.Getenv("CLOUDFLARE_ACCOUNT_ID"))
	if account == "" {
		return nil, fmt.Errorf("%w: CLOUDFLARE_ACCOUNT_ID is not set", jev.ErrConfig)
	}
	if !accountIDPattern.MatchString(account) {
		return nil, fmt.Errorf("%w: CLOUDFLARE_ACCOUNT_ID must be 32 hexadecimal characters", jev.ErrConfig)
	}
	base, err := clientkit.ValidateBaseURL(strings.TrimSpace(os.Getenv("CLOUDFLARE_BASE_URL")), defaultBaseURL)
	if err != nil {
		return nil, err
	}
	logger, err := clientkit.NewLogger(cfg.logOutput, "CLOUDFLARE_LOG_LEVEL", token)
	if err != nil {
		return nil, err
	}

	rt := NewTransport(http.DefaultTransport)
	httpClient := &http.Client{Transport: rt, CheckRedirect: clientkit.NoRedirect}
	if cfg.cacheDir != "" {
		httpClient = clientkit.NewCachingClient(cfg.cacheDir, rt)
	}
	jevOpts := []jev.Option{
		jev.WithAPIKey(token),
		jev.WithBaseURL(base + accountPath(account)),
		jev.WithModel(modelName),
		jev.WithRetry(defaultRetryPolicy()),
		jev.WithLogger(logger),
		jev.WithHTTPClient(httpClient),
	}
	if cfg.timeout > 0 {
		jevOpts = append(jevOpts, jev.WithTimeout(cfg.timeout))
	}
	return jev.New(jevOpts...)
}
