// Package eval scores labelled Jev choice results, so that the confidence
// thresholds can be judged on evidence instead of assumed.
//
// A [Result] is one answered decision together with its truth: whether the
// decision was dominant (exactly one option satisfies every constraint) or
// contested (several do), and for a dominant one the correct option. A result
// is safe to fast-path only when the decision is dominant and Jev picked the
// correct option. [Compute] turns a set of results into a [Report]: accuracy,
// how well confidence detects contested decisions, calibration, and the
// precision and recall of a fast path at each threshold.
//
// A metric that cannot be computed for a set is absent, never zero; see
// [Metric].
package eval

import (
	"errors"
	"fmt"

	"github.com/rshade/jev-decide/decision"
)

// ErrInvalidResult is wrapped by the error [NewResult] returns.
var ErrInvalidResult = errors.New("eval: invalid result")

// Class is whether a decision has one objectively correct option. The zero
// value is not a valid class.
type Class int

// The two classes of a labelled decision.
const (
	Dominant Class = iota + 1
	Contested
)

// ParseClass returns the class named "dominant" or "contested".
func ParseClass(s string) (Class, error) {
	switch s {
	case "dominant":
		return Dominant, nil
	case "contested":
		return Contested, nil
	default:
		return 0, fmt.Errorf("%w: class %q is neither dominant nor contested", ErrInvalidResult, s)
	}
}

func (c Class) String() string {
	switch c {
	case Dominant:
		return "dominant"
	case Contested:
		return "contested"
	default:
		return "invalid"
	}
}

// Result is one labelled answer. Create one with [NewResult].
type Result struct {
	class      Class
	pick       string
	confidence decision.Probability
	correct    string
}

// NewResult returns a labelled result. It fails when the class or confidence
// is not valid, when the pick is empty, when a dominant result has no correct
// option, and when a contested result has one.
func NewResult(class Class, pick string, confidence decision.Probability, correct string) (Result, error) {
	switch {
	case class != Dominant && class != Contested:
		return Result{}, fmt.Errorf("%w: class is not set", ErrInvalidResult)
	case !confidence.Valid():
		return Result{}, fmt.Errorf("%w: confidence is not a valid probability", ErrInvalidResult)
	case pick == "":
		return Result{}, fmt.Errorf("%w: pick is empty", ErrInvalidResult)
	case class == Dominant && correct == "":
		return Result{}, fmt.Errorf("%w: a dominant result needs a correct option", ErrInvalidResult)
	case class == Contested && correct != "":
		return Result{}, fmt.Errorf("%w: a contested result has no correct option, got %q", ErrInvalidResult, correct)
	}
	return Result{class: class, pick: pick, confidence: confidence, correct: correct}, nil
}

// Class returns the decision's class.
func (r Result) Class() Class { return r.class }

// Pick returns the option Jev picked. It is a pick, not a decision: see
// [OutcomeOf] for whether it clears the thresholds.
func (r Result) Pick() string { return r.pick }

// Confidence returns the pick confidence.
func (r Result) Confidence() decision.Probability { return r.confidence }

// Correct returns the correct option of a dominant decision, or "" for a
// contested one.
func (r Result) Correct() string { return r.correct }

// Safe reports whether the result is safe to fast-path: the decision is
// dominant and the pick is its correct option.
func (r Result) Safe() bool {
	return r.class == Dominant && r.pick == r.correct
}

// Outcome is which result a choice question with the same confidence and
// thresholds would produce.
type Outcome int

// The three outcomes, in the order of [decision.Result]. The zero value is
// escalate, so an outcome that was never set never reads as decided.
const (
	Escalate Outcome = iota
	Uncertain
	Decided
)

func (o Outcome) String() string {
	switch o {
	case Decided:
		return "decided"
	case Uncertain:
		return "uncertain"
	default:
		return "escalate"
	}
}

// OutcomeOf applies the rule [decision.Choose] uses: decided at or above the
// confident level, uncertain at or above the floor, escalate below it. An
// invalid confidence or thresholds escalate.
func OutcomeOf(confidence decision.Probability, th decision.Thresholds) Outcome {
	switch {
	case !confidence.Valid() || !th.Valid():
		return Escalate
	case confidence.AtLeast(th.Confident()):
		return Decided
	case confidence.AtLeast(th.Floor()):
		return Uncertain
	default:
		return Escalate
	}
}
