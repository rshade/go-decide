package decision

import (
	"fmt"
	"maps"
)

// ScoreResult is what [Rate] returns for a usable answer: exactly one of
// [ScoreDecided], [ScoreUncertain] or [ScoreEscalate]. The set is closed, so
// only this package creates results. Consume one with [MatchScore], which
// requires a handler for each case.
//
// Every result carries the fractional score, the nearest level, the answer's
// confidence and the probability of every level. Only [ScoreDecided] has a
// Level; the nearest level of the other two is not a rating.
type ScoreResult[T ~string] interface {
	// Score is the fractional position along the rubric, counting levels from 0.
	Score() float64
	// Nearest is the level closest to Score, with halves rounding down. On a
	// [ScoreUncertain] or [ScoreEscalate] result it is context, not a rating.
	Nearest() T
	// Confidence is the answer's confidence in its score.
	Confidence() Probability
	// Probabilities returns a copy of the probability of every level.
	Probabilities() map[T]Probability
	isScoreResult()
}

// ScoreDecided is a confident rating: confidence at or above the confident
// level.
type ScoreDecided[T ~string] interface {
	ScoreResult[T]
	// Level is the level a caller may act on.
	Level() T
	isScoreDecided()
}

// ScoreUncertain is a rating below the confident level and at or above the
// floor. It is meant for human review and has no Level.
type ScoreUncertain[T ~string] interface {
	ScoreResult[T]
	isScoreUncertain()
}

// ScoreEscalate is a rating below the floor. It is meant for the full debate
// and has no Level.
type ScoreEscalate[T ~string] interface {
	ScoreResult[T]
	isScoreEscalate()
}

type scoreBase[T ~string] struct {
	score         float64
	nearest       T
	confidence    Probability
	probabilities map[T]Probability
}

func (b scoreBase[T]) Score() float64 { return b.score }

func (b scoreBase[T]) Nearest() T { return b.nearest }

func (b scoreBase[T]) Confidence() Probability { return b.confidence }

func (b scoreBase[T]) Probabilities() map[T]Probability { return maps.Clone(b.probabilities) }

func (b scoreBase[T]) isScoreResult() {}

func (b scoreBase[T]) describe(name, field string) string {
	confidence := "unset"
	if b.confidence.Valid() {
		confidence = fmt.Sprintf("%.2f", b.confidence.Float64())
	}
	return fmt.Sprintf("%s %s=%s score=%.2f confidence=%s", name, field, b.nearest, b.score, confidence)
}

type scoreDecidedResult[T ~string] struct{ scoreBase[T] }

func (d scoreDecidedResult[T]) Level() T { return d.nearest }

func (d scoreDecidedResult[T]) String() string { return d.describe("decided", "level") }

func (scoreDecidedResult[T]) isScoreDecided() {}

type scoreUncertainResult[T ~string] struct{ scoreBase[T] }

func (scoreUncertainResult[T]) isScoreUncertain() {}

func (u scoreUncertainResult[T]) String() string { return u.describe("uncertain", "nearest") }

type scoreEscalateResult[T ~string] struct{ scoreBase[T] }

func (scoreEscalateResult[T]) isScoreEscalate() {}

func (e scoreEscalateResult[T]) String() string { return e.describe("escalate", "nearest") }

func newScoreResult[T ~string](k kind, score float64, nearest T, confidence Probability, probabilities map[T]Probability) ScoreResult[T] {
	b := scoreBase[T]{score: score, nearest: nearest, confidence: confidence, probabilities: maps.Clone(probabilities)}
	switch k {
	case kindDecided:
		return scoreDecidedResult[T]{b}
	case kindUncertain:
		return scoreUncertainResult[T]{b}
	default:
		return scoreEscalateResult[T]{b}
	}
}

// MatchScore runs the handler for r's case and returns its value. It needs a
// handler for each of the three cases, so leaving one out does not compile.
//
// A nil ScoreResult runs onEscalate with an empty [ScoreEscalate] value. It
// never runs onDecided.
func MatchScore[T ~string, R any](
	r ScoreResult[T],
	onDecided func(ScoreDecided[T]) R,
	onUncertain func(ScoreUncertain[T]) R,
	onEscalate func(ScoreEscalate[T]) R,
) R {
	switch v := r.(type) {
	case ScoreDecided[T]:
		return onDecided(v)
	case ScoreUncertain[T]:
		return onUncertain(v)
	case ScoreEscalate[T]:
		return onEscalate(v)
	default:
		return onEscalate(scoreEscalateResult[T]{})
	}
}
