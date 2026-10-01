package decision

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"github.com/kataras/jev"

	"github.com/rshade/jev-decide/jevclient"
)

const scoreQuestionName = "score"

// RateQuestion is one score question: the content it is about, what to rate,
// and the rubric to rate it against.
type RateQuestion[T ~string] struct {
	// State is the content the question refers to: a string, an object or a
	// list.
	State any
	// Instructions says what to rate.
	Instructions string
	// Levels is the validated rubric. The zero value is rejected.
	Levels Levels[T]
}

// Validate reports whether q can be asked, without sending anything. It rejects
// levels that were not built by [NewLevels], and the State checks of
// [Question.Validate]. Errors are *[FieldError] values.
func (q RateQuestion[T]) Validate() error {
	if !q.Levels.Valid() {
		return fmt.Errorf("%w: build the levels with NewLevels", ErrInvalidLevels)
	}
	return validateState(q.State, len(q.Instructions)+q.Levels.sizeBytes())
}

// Rate asks Jev the score question and returns a [ScoreResult]. A nil client,
// an invalid question (see [RateQuestion.Validate]) and invalid thresholds are
// reported before any request is sent, so they cost nothing.
func Rate[T ~string](ctx context.Context, client *jev.Client, q RateQuestion[T], opts ...ChooseOption) (ScoreResult[T], error) {
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

	var instructions any
	if q.Instructions != "" {
		instructions = q.Instructions
	}
	resp, err := client.SystemOne(ctx, jev.Request{
		State: q.State,
		Questions: jev.Questions{
			scoreQuestionName: jev.Score{Instructions: instructions, Criteria: q.Levels.criteria()},
		},
	})
	if err != nil {
		return nil, jevclient.Classify(ctx, err)
	}
	answer, ok := resp.Score(scoreQuestionName)
	if !ok {
		return nil, invalidResponse(ctx, "no score answer")
	}
	return scoreFromAnswer(ctx, answer, q.Levels, cfg.thresholds)
}

func scoreFromAnswer[T ~string](ctx context.Context, answer jev.ScoreAnswer, levels Levels[T], th Thresholds) (ScoreResult[T], error) {
	top := float64(levels.Len() - 1)
	if math.IsNaN(answer.Score) || answer.Score < 0 || answer.Score > top {
		return nil, invalidResponse(ctx, "score %v is outside the rubric, which runs from 0 to %v", answer.Score, top)
	}
	confidence, err := NewProbability(answer.Confidence)
	if err != nil {
		return nil, invalidResponse(ctx, "confidence: %v", err)
	}
	probabilities := make(map[T]Probability, levels.Len())
	for i, name := range levels.Names() {
		p, ok := answer.Probabilities[strconv.Itoa(i)]
		if !ok {
			return nil, invalidResponse(ctx, "no probability for level %q", name)
		}
		if probabilities[name], err = NewProbability(p); err != nil {
			return nil, invalidResponse(ctx, "probability of level %q: %v", name, err)
		}
	}
	nearest, _ := levels.At(nearestIndex(answer.Score))
	return newScoreResult(classify(confidence, th), answer.Score, nearest.Name, confidence, probabilities), nil
}

// nearestIndex rounds score to the closest level, with halves rounding down so
// an in-between value never reads as a step up.
func nearestIndex(score float64) int { return int(math.Ceil(score - 0.5)) }
