package decision

import (
	"errors"
	"fmt"
)

// ErrInvalidThresholds is wrapped by the error [NewThresholds] returns when a
// level is not valid or the floor is not below the confident level.
var ErrInvalidThresholds = errors.New("decision: invalid thresholds")

// Thresholds are the two confidence levels that split a choice answer into a
// [Decided], an [Uncertain] and an [Escalate] result. The zero value is not
// valid.
type Thresholds struct {
	floor, confident Probability
}

// NewThresholds returns thresholds with the given floor and confident level.
// Both must be valid and the floor must be below the confident level.
func NewThresholds(floor, confident Probability) (Thresholds, error) {
	if !floor.Valid() || !confident.Valid() {
		return Thresholds{}, fmt.Errorf("%w: both levels must be valid probabilities", ErrInvalidThresholds)
	}
	if !floor.Less(confident) {
		return Thresholds{}, fmt.Errorf("%w: floor %v must be below confident level %v",
			ErrInvalidThresholds, floor.Float64(), confident.Float64())
	}
	return Thresholds{floor: floor, confident: confident}, nil
}

// DefaultThresholds returns a floor of 0.5 and a confident level of 0.9.
//
// Both are placeholders until they are tuned on real decisions. The confident
// level comes from the spike, which found clear decisions near 0.98 and
// contested ones near 0.58; the floor has no data behind it.
func DefaultThresholds() Thresholds {
	return Thresholds{
		floor:     Probability{v: 0.5, set: true},
		confident: Probability{v: 0.9, set: true},
	}
}

// Valid reports whether t was created by [NewThresholds] or
// [DefaultThresholds].
func (t Thresholds) Valid() bool { return t.floor.Valid() && t.confident.Valid() }

// Floor returns the level below which an answer is escalated.
func (t Thresholds) Floor() Probability { return t.floor }

// Confident returns the level at or above which an answer is decided.
func (t Thresholds) Confident() Probability { return t.confident }
