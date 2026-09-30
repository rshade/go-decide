package decision

import (
	"errors"
	"fmt"
)

// ErrInvalidQuestion is wrapped by the error [Question.Validate] returns for a
// question that cannot be asked.
var ErrInvalidQuestion = errors.New("decision: invalid question")

// FieldError reports which field of the input was rejected and why. It unwraps
// to the sentinel for the kind of input: [ErrInvalidOptions] or
// [ErrInvalidQuestion].
type FieldError struct {
	// Field is the rejected field, such as "State" or "Options".
	Field string
	// Reason says what is wrong with it.
	Reason string

	kind error
}

func newFieldError(kind error, field, format string, args ...any) *FieldError {
	return &FieldError{Field: field, Reason: fmt.Sprintf(format, args...), kind: kind}
}

func (e *FieldError) Error() string {
	return fmt.Sprintf("%v: %s: %s", e.kind, e.Field, e.Reason)
}

func (e *FieldError) Unwrap() error { return e.kind }
