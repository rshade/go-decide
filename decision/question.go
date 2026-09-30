package decision

import (
	"encoding/json"
	"fmt"
)

const (
	// MaxOptions is the most options a choice question can offer.
	MaxOptions = 255

	// maxStateTokens is the documented limit for state plus the longest question.
	maxStateTokens = 32_000

	// bytesPerToken is deliberately generous: JSON runs denser than four bytes a
	// token, so the estimate errs toward sending the question.
	bytesPerToken = 4
)

// Validate reports whether q can be asked, without sending anything. It rejects
// options that were not built by [NewOptions], a State that is nil or empty or
// cannot be encoded as JSON, and a question whose estimated size clearly
// exceeds the API limit for state plus the question. Errors are *[FieldError]
// values.
func (q Question[T]) Validate() error {
	if !q.Options.Valid() {
		return fmt.Errorf("%w: build the options with NewOptions", ErrInvalidOptions)
	}
	payload, err := json.Marshal(q.State)
	if err != nil {
		return newFieldError(ErrInvalidQuestion, "State", "cannot be encoded as JSON: %v", err)
	}
	switch string(payload) {
	case "null", `""`, "{}", "[]":
		return newFieldError(ErrInvalidQuestion, "State", "is empty (%s)", payload)
	}
	if tokens := q.estimateTokens(len(payload)); tokens > maxStateTokens {
		return newFieldError(ErrInvalidQuestion, "State",
			"is too large: about %d tokens with the question, the limit is %d", tokens, maxStateTokens)
	}
	return nil
}

func (q Question[T]) estimateTokens(stateBytes int) int {
	size := stateBytes + len(q.Instructions)
	for _, name := range q.Options.Names() {
		description, _ := q.Options.Description(name)
		size += len(name) + len(description)
	}
	return size / bytesPerToken
}
