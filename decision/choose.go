package decision

import (
	"context"
	"errors"
	"fmt"

	"github.com/kataras/jev"

	"github.com/rshade/go-decide/jevclient"
)

// ErrNilClient is returned by [Choose] when it is given no client.
var ErrNilClient = errors.New("decision: nil client")

const questionName = "choice"

// Question is one choice question: the content it is about, what to decide, and
// the options to pick from.
type Question[T ~string] struct {
	// State is the content the question refers to: a string, an object or a
	// list.
	State any
	// Instructions says what to decide.
	Instructions string
	// Options is the validated set of choices. The zero value is rejected.
	Options Options[T]
}

// ChooseOption adjusts how [Choose] runs.
type ChooseOption func(*chooseConfig)

type chooseConfig struct {
	thresholds Thresholds
}

// WithThresholds replaces the default thresholds. A thresholds value that is not
// valid makes [Choose] fail before any request is sent.
func WithThresholds(t Thresholds) ChooseOption {
	return func(c *chooseConfig) { c.thresholds = t }
}

// Choose asks Jev the question and returns a [Result]. A nil client, an invalid
// question (see [Question.Validate]) and invalid thresholds are reported before
// any request is sent, so they cost nothing.
func Choose[T ~string](ctx context.Context, client *jev.Client, q Question[T], opts ...ChooseOption) (Result[T], error) {
	cfg := chooseConfig{thresholds: DefaultThresholds()}
	for _, opt := range opts {
		opt(&cfg)
	}
	if client == nil {
		return nil, ErrNilClient
	}
	if err := q.Validate(); err != nil {
		return nil, err
	}
	if !cfg.thresholds.Valid() {
		return nil, fmt.Errorf("%w: build the thresholds with NewThresholds or DefaultThresholds", ErrInvalidThresholds)
	}

	criteria := make(map[string]string, q.Options.Len())
	for _, name := range q.Options.Names() {
		description, _ := q.Options.Description(name)
		criteria[string(name)] = description
	}
	var instructions any
	if q.Instructions != "" {
		instructions = q.Instructions
	}
	resp, err := client.SystemOne(ctx, jev.Request{
		State: q.State,
		Questions: jev.Questions{
			questionName: jev.Choice{Instructions: instructions, Criteria: criteria},
		},
	})
	if err != nil {
		return nil, jevclient.Classify(ctx, err)
	}

	return fromResponse(ctx, resp, q.Options, cfg.thresholds)
}

// fromResponse finds the answer to the question in resp and turns it into a
// result. The client library already guarantees the answer is present and a
// choice; checking again keeps a change in that pinned dependency from turning a
// missing answer into a decision.
func fromResponse[T ~string](ctx context.Context, resp *jev.Response, options Options[T], th Thresholds) (Result[T], error) {
	answer, ok := resp.Choice(questionName)
	if !ok {
		return nil, invalidResponse(ctx, "no choice answer")
	}
	return fromAnswer(ctx, answer, options, th)
}

// fromAnswer turns an answer into a result. The client library already rejects
// most malformed answers; checking again here means a change in that pinned
// dependency cannot let an unusable answer become a decision.
func fromAnswer[T ~string](ctx context.Context, answer jev.ChoiceAnswer, options Options[T], th Thresholds) (Result[T], error) {
	leading := T(answer.Choice)
	if !options.Contains(leading) {
		return nil, invalidResponse(ctx, "%q is not one of the options", answer.Choice)
	}
	confidence, err := NewProbability(answer.Confidence)
	if err != nil {
		return nil, invalidResponse(ctx, "confidence: %v", err)
	}
	probabilities := make(map[T]Probability, options.Len())
	for _, name := range options.Names() {
		p, ok := answer.Probabilities[string(name)]
		if !ok {
			return nil, invalidResponse(ctx, "no probability for %q", name)
		}
		if probabilities[name], err = NewProbability(p); err != nil {
			return nil, invalidResponse(ctx, "probability of %q: %v", name, err)
		}
	}
	return newResult(classify(confidence, th), leading, confidence, probabilities), nil
}

func invalidResponse(ctx context.Context, format string, args ...any) error {
	return jevclient.Classify(ctx, fmt.Errorf("%w: %s", jev.ErrResponse, fmt.Sprintf(format, args...)))
}
