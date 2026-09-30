package jevclient

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
)

const redacted = "[REDACTED]"

// Error codes set by [Classify]. The set is closed: callers can switch on it.
const (
	CodeUnauthorized    = "jev.unauthorized"
	CodeInvalidRequest  = "jev.invalid_request"
	CodeRateLimited     = "jev.rate_limited"
	CodeOverloaded      = "jev.overloaded"
	CodeStatus          = "jev.status"
	CodeTimeout         = "jev.timeout"
	CodeConnection      = "jev.connection"
	CodeInvalidResponse = "jev.invalid_response"
	CodeInternal        = "jev.internal"
)

// Classify converts a failure from a Jev call into a [*contract.Error].
// A cancellation or deadline set by the caller is returned unchanged.
// The API key is redacted from every string field of the result. The key is
// read from TYPESAFE_API_KEY when Classify runs, so the variable must still
// hold the key the client was built with.
func Classify(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if isCallerCancellation(err) {
		return err
	}
	ce := classify(ctx, err)
	redactKey(ce, strings.TrimSpace(os.Getenv("TYPESAFE_API_KEY")))
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
	return contract.NewError(ctx, CodeInternal, err.Error())
}

func redactKey(ce *contract.Error, key string) {
	if key == "" {
		return
	}
	ce.Message = strings.ReplaceAll(ce.Message, key, redacted)
	for name, value := range ce.Context {
		if text, ok := value.(string); ok {
			ce.Context[name] = strings.ReplaceAll(text, key, redacted)
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
	case http.StatusUnauthorized:
		code, exit, retryable = CodeUnauthorized, contract.ExitAuth, ptr(false)
	case http.StatusUnprocessableEntity:
		code, exit, retryable = CodeInvalidRequest, contract.ExitValidation, ptr(false)
	case http.StatusTooManyRequests:
		code, exit, retryable = CodeRateLimited, contract.ExitNetwork, ptr(true)
	case statusOverloaded:
		code, exit, retryable = CodeOverloaded, contract.ExitNetwork, ptr(true)
	}
	fields := map[string]any{"status": apiErr.Status}
	if apiErr.RequestID != "" {
		fields["request_id"] = apiErr.RequestID
	}
	if errorType := detailErrorType(apiErr.Body); errorType != "" {
		fields["error_type"] = errorType
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
	return contract.NewError(ctx, code, apiErr.Message, opts...)
}

func detailErrorType(body []byte) string {
	var payload struct {
		Detail struct {
			ErrorType string `json:"error_type"`
		} `json:"detail"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return ""
	}
	return payload.Detail.ErrorType
}

func ptr[T any](v T) *T { return &v }
