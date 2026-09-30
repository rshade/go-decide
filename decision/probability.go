package decision

import (
	"errors"
	"fmt"
	"math"
)

// ErrInvalidProbability is wrapped by the error [NewProbability] returns for a
// number that is not from 0 to 1.
var ErrInvalidProbability = errors.New("decision: invalid probability")

// Probability is a number from 0 to 1. Create one with [NewProbability]: the
// zero value is not a valid probability, so a value that was never set cannot
// be mistaken for a probability of 0.
type Probability struct {
	v   float64
	set bool
}

// NewProbability returns f as a Probability. It fails for NaN, infinities and
// any number below 0 or above 1.
func NewProbability(f float64) (Probability, error) {
	if math.IsNaN(f) || f < 0 || f > 1 {
		return Probability{}, fmt.Errorf("%w: %v is not from 0 to 1", ErrInvalidProbability, f)
	}
	return Probability{v: f, set: true}, nil
}

// Valid reports whether p was created by [NewProbability].
func (p Probability) Valid() bool { return p.set }

// Float64 returns the number p holds, or 0 when p is not valid.
func (p Probability) Float64() float64 { return p.v }

// AtLeast reports whether p is greater than or equal to other. It is false
// when either side is not valid.
func (p Probability) AtLeast(other Probability) bool {
	return p.set && other.set && p.v >= other.v
}

// Less reports whether p is smaller than other. It is false when either side
// is not valid.
func (p Probability) Less(other Probability) bool {
	return p.set && other.set && p.v < other.v
}
