package clefclient

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"os"
	"strings"

	"github.com/kataras/jev"
	"github.com/rshade/ax-go/contract"

	"github.com/rshade/go-decide/internal/clientkit"
)

// Error codes set by [Classify]. The set is closed: callers can switch on it.
const (
	CodeUnauthorized    = "clef.unauthorized"
	CodeInvalidRequest  = "clef.invalid_request"
	CodeRateLimited     = "clef.rate_limited"
	CodeStatus          = "clef.status"
	CodeTimeout         = "clef.timeout"
	CodeConnection      = "clef.connection"
	CodeInvalidResponse = "clef.invalid_response"
	CodeInternal        = "clef.internal"
)

// Classify converts a failure from a clef call into a [*contract.Error].
// A cancellation or deadline set by the caller is returned unchanged.
// The token is redacted from every string field of the result. It is read
// from CLOUDFLARE_AUTH_TOKEN when Classify runs, so the variable must still
// hold the token the client was built with.
func Classify(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if isCallerCancellation(err) {
		return err
	}
	ce := classify(ctx, err)
	redactToken(ce, strings.TrimSpace(os.Getenv("CLOUDFLARE_AUTH_TOKEN")))
	return ce
}

func classify(ctx context.Context, err error) *contract.Error {
	var apiErr *jev.APIError
	if errors.As(err, &apiErr) {
		return classifyAPIError(ctx, apiErr)
	}
	if errors.Is(err, jev.ErrResponse) {
		return contract.NewError(ctx, CodeInvalidResponse, err.Error())
	}
	if errors.Is(err, jev.ErrRequest) {
		return contract.NewError(ctx, CodeInvalidRequest, err.Error(),
			contract.WithErrorExitCode(contract.ExitValidation),
			contract.WithRetryable(false))
	}
	if errors.Is(err, jev.ErrConnection) {
		code := CodeConnection
		if errors.Is(err, jev.ErrTimeout) {
			code = CodeTimeout
		}
		return contract.NewError(ctx, code, err.Error(),
			contract.WithErrorExitCode(contract.ExitNetwork),
			contract.WithRetryable(true))
	}
	if errors.Is(err, jev.ErrConfig) {
		msg := strings.Replace(err.Error(), jev.ErrConfig.Error(), "clef: invalid configuration", 1)
		return contract.NewError(ctx, CodeInternal, msg)
	}
	return contract.NewError(ctx, CodeInternal, err.Error())
}

func redactToken(ce *contract.Error, token string) {
	if token == "" {
		return
	}
	ce.Message = strings.ReplaceAll(ce.Message, token, clientkit.Redacted)
	for name, value := range ce.Context {
		if text, ok := value.(string); ok {
			ce.Context[name] = strings.ReplaceAll(text, token, clientkit.Redacted)
		}
	}
}

func isCallerCancellation(err error) bool {
	if errors.Is(err, context.Canceled) {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, jev.ErrTimeout)
}

func classifyAPIError(ctx context.Context, apiErr *jev.APIError) *contract.Error {
	code, exit, retryable := CodeStatus, contract.ExitInternal, (*bool)(nil)
	switch apiErr.Status {
	case http.StatusUnauthorized, http.StatusForbidden:
		code, exit, retryable = CodeUnauthorized, contract.ExitAuth, ptr(false)
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		code, exit, retryable = CodeInvalidRequest, contract.ExitValidation, ptr(false)
	case http.StatusTooManyRequests:
		code, exit, retryable = CodeRateLimited, contract.ExitNetwork, ptr(true)
	}
	fields := map[string]any{"status": apiErr.Status}
	if id := apiErr.Header.Get("cf-ai-req-id"); id != "" {
		fields["request_id"] = id
	}
	message := apiErr.Message
	if first, ok := firstCloudflareError(apiErr.Body); ok {
		fields["cloudflare_code"] = first.Code
		if first.Message != "" {
			message = first.Message
		}
	}
	opts := []contract.ErrorOption{
		contract.WithErrorExitCode(exit),
		contract.WithErrorContext(fields),
	}
	if retryable != nil {
		opts = append(opts, contract.WithRetryable(*retryable))
		if delay, ok := apiErr.RetryAfter(); ok && *retryable && delay > 0 {
			opts = append(opts, contract.WithRetryAfterSeconds(int64(math.Ceil(delay.Seconds()))))
		}
	}
	return contract.NewError(ctx, code, message, opts...)
}

type cloudflareError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func firstCloudflareError(body []byte) (cloudflareError, bool) {
	var payload struct {
		Errors []cloudflareError `json:"errors"`
	}
	if json.Unmarshal(body, &payload) != nil || len(payload.Errors) == 0 {
		return cloudflareError{}, false
	}
	return payload.Errors[0], true
}

func ptr[T any](v T) *T { return &v }
