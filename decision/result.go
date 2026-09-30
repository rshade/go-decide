package decision

import (
	"fmt"
	"maps"
)

// Result is what [Choose] returns for a usable answer: exactly one of
// [Decided], [Uncertain] or [Escalate]. The set is closed, so only this package
// creates results. Consume one with [Match], which requires a handler for each
// case, or with a type switch over the three.
//
// Every result carries the leading option, the answer's confidence and the
// probability of every option. Only [Decided] has a Choice; the leading option
// of the other two is not a decision.
type Result[T ~string] interface {
	// Confidence is the answer's confidence in its leading option.
	Confidence() Probability
	// Leading is the option Jev rated most likely. On an [Uncertain] or
	// [Escalate] result it is context for a person or a debate, not a decision.
	Leading() T
	// Probabilities returns a copy of the probability of every option.
	Probabilities() map[T]Probability
	isResult()
}

// Decided is a confident answer: confidence at or above the confident level.
// It is clear enough to skip the debate, which is not the same as approved.
type Decided[T ~string] interface {
	Result[T]
	// Choice is the option a caller may act on.
	Choice() T
	isDecided()
}

// Uncertain is an answer below the confident level and at or above the floor.
// It is meant for human review and has no Choice.
type Uncertain[T ~string] interface {
	Result[T]
	isUncertain()
}

// Escalate is an answer below the floor. It is meant for the full debate and
// has no Choice.
type Escalate[T ~string] interface {
	Result[T]
	isEscalate()
}

type base[T ~string] struct {
	leading       T
	confidence    Probability
	probabilities map[T]Probability
}

func (b base[T]) Confidence() Probability { return b.confidence }

func (b base[T]) Leading() T { return b.leading }

func (b base[T]) Probabilities() map[T]Probability { return maps.Clone(b.probabilities) }

func (b base[T]) isResult() {}

// describe labels the option "choice" only for a decided result, so a log line
// never presents a non-decision as one.
func (b base[T]) describe(name, field string) string {
	confidence := "unset"
	if b.confidence.Valid() {
		confidence = fmt.Sprintf("%.2f", b.confidence.Float64())
	}
	return fmt.Sprintf("%s %s=%s confidence=%s", name, field, b.leading, confidence)
}

type decidedResult[T ~string] struct{ base[T] }

func (d decidedResult[T]) Choice() T { return d.leading }

func (d decidedResult[T]) String() string { return d.describe("decided", "choice") }

func (decidedResult[T]) isDecided() {}

type uncertainResult[T ~string] struct{ base[T] }

func (uncertainResult[T]) isUncertain() {}

func (u uncertainResult[T]) String() string { return u.describe("uncertain", "leading") }

type escalateResult[T ~string] struct{ base[T] }

func (escalateResult[T]) isEscalate() {}

func (e escalateResult[T]) String() string { return e.describe("escalate", "leading") }

func newResult[T ~string](k kind, leading T, confidence Probability, probabilities map[T]Probability) Result[T] {
	b := base[T]{leading: leading, confidence: confidence, probabilities: maps.Clone(probabilities)}
	switch k {
	case kindDecided:
		return decidedResult[T]{b}
	case kindUncertain:
		return uncertainResult[T]{b}
	default:
		return escalateResult[T]{b}
	}
}
