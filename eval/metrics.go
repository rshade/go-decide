package eval

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/rshade/jev-decide/decision"
)

// ErrNoResults is returned by [Compute] for an empty result set.
var ErrNoResults = errors.New("eval: no results")

// The grid runs from 1/gridDivisions to (gridDivisions-1)/gridDivisions:
// 0.05 to 0.95 in steps of 0.05, built from integers so every value is exact.
const (
	gridDivisions = 20
	gridSteps     = gridDivisions - 1
)

// Metric is a number that may be undefined for a set, such as accuracy over
// a set with no dominant decisions. The zero value is absent, so an undefined
// metric never reads as 0. It marshals to JSON as the number or null.
type Metric struct {
	v  float64
	ok bool
}

func metricOf(v float64) Metric { return Metric{v: v, ok: true} }

// ratio returns num/den, or an absent metric when den is zero.
func ratio(num, den int) Metric {
	if den == 0 {
		return Metric{}
	}
	return metricOf(float64(num) / float64(den))
}

// Get returns the value and whether it is defined.
func (m Metric) Get() (float64, bool) { return m.v, m.ok }

// MarshalJSON writes the value, or null when it is absent.
func (m Metric) MarshalJSON() ([]byte, error) {
	if !m.ok {
		return []byte("null"), nil
	}
	return json.Marshal(m.v)
}

// Row is the fast path at one threshold: the results with a confidence at or
// above it, how many of those are safe, and the precision and recall.
type Row struct {
	Threshold decision.Probability
	Selected  int
	Safe      int
	Unsafe    int
	// Precision is safe among selected; absent when nothing is selected.
	Precision Metric
	// Recall is selected among all safe results; absent when none is safe.
	Recall Metric
}

// Counts are how many results each outcome would get under the run's
// thresholds, and how many decided ones are not safe to fast-path.
type Counts struct {
	Decided       int
	Uncertain     int
	Escalate      int
	DecidedUnsafe int
}

// Report is what [Compute] measures over a result set.
type Report struct {
	// Accuracy is correct picks over dominant decisions; absent with none.
	Accuracy Metric
	// ContestedAUC is the area under the ROC curve for detecting contested
	// decisions from 1 minus the pick confidence; absent unless both classes
	// are present.
	ContestedAUC Metric
	// Brier is the mean squared difference between the pick confidence and 1
	// for a safe result, 0 for any other.
	Brier float64
	// ByThreshold has one row per threshold, ascending.
	ByThreshold []Row
	// Outcomes counts the results under the run's thresholds.
	Outcomes Counts
}

// Compute measures results under th. It fails for an empty set and for
// thresholds that are not valid.
func Compute(results []Result, th decision.Thresholds) (Report, error) {
	if len(results) == 0 {
		return Report{}, ErrNoResults
	}
	if !th.Valid() {
		return Report{}, fmt.Errorf("%w: thresholds are not set", decision.ErrInvalidThresholds)
	}
	rows, err := byThreshold(results, th)
	if err != nil {
		return Report{}, err
	}
	return Report{
		Accuracy:     accuracy(results),
		ContestedAUC: contestedAUC(results),
		Brier:        brier(results),
		ByThreshold:  rows,
		Outcomes:     outcomes(results, th),
	}, nil
}

func accuracy(results []Result) Metric {
	var dominant, correct int
	for _, r := range results {
		if r.class == Dominant {
			dominant++
			if r.Safe() {
				correct++
			}
		}
	}
	return ratio(correct, dominant)
}

// contestedAUC compares every contested result with every dominant one.
// Scoring by 1 minus confidence, a contested result ranks above a dominant one
// exactly when its confidence is lower, so the confidences are compared
// directly and no subtraction rounds a tie apart.
func contestedAUC(results []Result) Metric {
	var pairs int
	var wins float64
	for _, pos := range results {
		if pos.class != Contested {
			continue
		}
		for _, neg := range results {
			if neg.class != Dominant {
				continue
			}
			pairs++
			switch p, n := pos.confidence.Float64(), neg.confidence.Float64(); {
			case p < n:
				wins++
			case p == n:
				wins += 0.5
			}
		}
	}
	if pairs == 0 {
		return Metric{}
	}
	return metricOf(wins / float64(pairs))
}

func brier(results []Result) float64 {
	var sum float64
	for _, r := range results {
		target := 0.0
		if r.Safe() {
			target = 1
		}
		d := r.confidence.Float64() - target
		sum += d * d
	}
	return sum / float64(len(results))
}

// thresholdGrid returns 0.05 to 0.95 in steps of 0.05 plus the run's two
// levels, ascending and without duplicates.
func thresholdGrid(th decision.Thresholds) ([]decision.Probability, error) {
	values := make([]float64, 0, gridSteps+2)
	for i := 1; i <= gridSteps; i++ {
		values = append(values, float64(i)/gridDivisions)
	}
	values = append(values, th.Floor().Float64(), th.Confident().Float64())
	slices.Sort(values)
	values = slices.Compact(values)

	grid := make([]decision.Probability, 0, len(values))
	for _, v := range values {
		p, err := decision.NewProbability(v)
		if err != nil {
			return nil, fmt.Errorf("eval: threshold grid value %v: %w", v, err)
		}
		grid = append(grid, p)
	}
	return grid, nil
}

func byThreshold(results []Result, th decision.Thresholds) ([]Row, error) {
	var totalSafe int
	for _, r := range results {
		if r.Safe() {
			totalSafe++
		}
	}
	grid, err := thresholdGrid(th)
	if err != nil {
		return nil, err
	}
	rows := make([]Row, 0, len(grid))
	for _, t := range grid {
		row := Row{Threshold: t}
		for _, r := range results {
			if !r.confidence.AtLeast(t) {
				continue
			}
			row.Selected++
			if r.Safe() {
				row.Safe++
			} else {
				row.Unsafe++
			}
		}
		row.Precision = ratio(row.Safe, row.Selected)
		row.Recall = ratio(row.Safe, totalSafe)
		rows = append(rows, row)
	}
	return rows, nil
}

func outcomes(results []Result, th decision.Thresholds) Counts {
	var c Counts
	for _, r := range results {
		switch OutcomeOf(r.confidence, th) {
		case Decided:
			c.Decided++
			if !r.Safe() {
				c.DecidedUnsafe++
			}
		case Uncertain:
			c.Uncertain++
		default:
			c.Escalate++
		}
	}
	return c
}
