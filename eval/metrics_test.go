package eval

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/rshade/jev-decide/decision"
)

func dominant(t *testing.T, pick string, confidence float64) Result {
	t.Helper()
	r, err := NewResult(Dominant, pick, prob(t, confidence), "a")
	if err != nil {
		t.Fatalf("NewResult error: %v", err)
	}
	return r
}

func contested(t *testing.T, confidence float64) Result {
	t.Helper()
	r, err := NewResult(Contested, "a", prob(t, confidence), "")
	if err != nil {
		t.Fatalf("NewResult error: %v", err)
	}
	return r
}

func thresholds(t *testing.T, floor, confident float64) decision.Thresholds {
	t.Helper()
	th, err := decision.NewThresholds(prob(t, floor), prob(t, confident))
	if err != nil {
		t.Fatalf("NewThresholds error: %v", err)
	}
	return th
}

func compute(t *testing.T, results []Result, th decision.Thresholds) Report {
	t.Helper()
	r, err := Compute(results, th)
	if err != nil {
		t.Fatalf("Compute error: %v", err)
	}
	return r
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-12 }

func assertMetric(t *testing.T, name string, m Metric, want float64) {
	t.Helper()
	got, ok := m.Get()
	if !ok || !near(got, want) {
		t.Errorf("%s = %v (defined %v), want %v", name, got, ok, want)
	}
}

func assertAbsent(t *testing.T, name string, m Metric) {
	t.Helper()
	if got, ok := m.Get(); ok {
		t.Errorf("%s = %v, want absent", name, got)
	}
}

func TestComputeFailsWithoutResultsOrThresholds(t *testing.T) {
	if _, err := Compute(nil, decision.DefaultThresholds()); !errors.Is(err, ErrNoResults) {
		t.Errorf("Compute(nil) error = %v, want ErrNoResults", err)
	}
	results := []Result{dominant(t, "a", 0.9)}
	if _, err := Compute(results, decision.Thresholds{}); !errors.Is(err, decision.ErrInvalidThresholds) {
		t.Errorf("Compute with unset thresholds error = %v, want ErrInvalidThresholds", err)
	}
}

func TestContestedAUC(t *testing.T) {
	tests := []struct {
		name    string
		results []Result
		want    float64
	}{
		{"perfect separation", []Result{contested(t, 0.5), contested(t, 0.6), dominant(t, "a", 0.9), dominant(t, "a", 0.99)}, 1},
		{"reversed separation", []Result{contested(t, 0.95), dominant(t, "a", 0.5)}, 0},
		{"all ties", []Result{contested(t, 0.8), contested(t, 0.8), dominant(t, "a", 0.8)}, 0.5},
		{"one tie among wins", []Result{contested(t, 0.8), dominant(t, "a", 0.8), dominant(t, "a", 0.9)}, 0.75},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertMetric(t, "ContestedAUC", compute(t, tt.results, decision.DefaultThresholds()).ContestedAUC, tt.want)
		})
	}
}

func TestOnlyContestedLeavesAccuracyAndAUCAbsent(t *testing.T) {
	r := compute(t, []Result{contested(t, 0.6), contested(t, 0.7)}, decision.DefaultThresholds())
	assertAbsent(t, "Accuracy", r.Accuracy)
	assertAbsent(t, "ContestedAUC", r.ContestedAUC)
	if !near(r.Brier, (0.36+0.49)/2) {
		t.Errorf("Brier = %v, want %v", r.Brier, (0.36+0.49)/2)
	}
}

func TestBrierAndAccuracyOfAConfidentWrongPick(t *testing.T) {
	r := compute(t, []Result{dominant(t, "b", 0.9)}, decision.DefaultThresholds())
	if !near(r.Brier, 0.81) {
		t.Errorf("Brier = %v, want 0.81", r.Brier)
	}
	assertMetric(t, "Accuracy", r.Accuracy, 0)
}

func TestAccuracyCountsDominantOnly(t *testing.T) {
	r := compute(t, []Result{dominant(t, "a", 0.9), dominant(t, "b", 0.9), contested(t, 0.9)}, decision.DefaultThresholds())
	assertMetric(t, "Accuracy", r.Accuracy, 0.5)
}

func rowAt(t *testing.T, r Report, threshold float64) Row {
	t.Helper()
	for _, row := range r.ByThreshold {
		if row.Threshold.Float64() == threshold {
			return row
		}
	}
	t.Fatalf("no row for threshold %v", threshold)
	return Row{}
}

func TestByThreshold(t *testing.T) {
	t.Run("grid is ascending with the run's levels and no duplicates", func(t *testing.T) {
		r := compute(t, []Result{dominant(t, "a", 0.9)}, thresholds(t, 0.5, 0.92))
		if len(r.ByThreshold) != gridSteps+1 {
			t.Fatalf("%d rows, want %d: the 0.5 floor is on the grid, 0.92 is not", len(r.ByThreshold), gridSteps+1)
		}
		for i := 1; i < len(r.ByThreshold); i++ {
			if !r.ByThreshold[i-1].Threshold.Less(r.ByThreshold[i].Threshold) {
				t.Fatalf("rows %d and %d are not strictly ascending", i-1, i)
			}
		}
		for i, row := range r.ByThreshold {
			if row.Threshold.Float64() == 0.92 {
				if r.ByThreshold[i-1].Threshold.Float64() != 0.9 || r.ByThreshold[i+1].Threshold.Float64() != 0.95 {
					t.Fatal("0.92 is not between 0.9 and 0.95")
				}
				return
			}
		}
		t.Fatal("no 0.92 row")
	})

	t.Run("unsafe results above the level are counted", func(t *testing.T) {
		r := compute(t, []Result{dominant(t, "a", 0.95), contested(t, 0.96), dominant(t, "a", 0.5)}, decision.DefaultThresholds())
		row := rowAt(t, r, 0.95)
		if row.Selected != 2 || row.Safe != 1 || row.Unsafe != 1 {
			t.Fatalf("0.95 row = %+v, want 2 selected, 1 safe, 1 unsafe", row)
		}
		assertMetric(t, "Precision", row.Precision, 0.5)
		assertMetric(t, "Recall", row.Recall, 0.5)
	})

	t.Run("nothing selected leaves precision absent", func(t *testing.T) {
		r := compute(t, []Result{dominant(t, "a", 0.9)}, decision.DefaultThresholds())
		row := rowAt(t, r, 0.95)
		if row.Selected != 0 {
			t.Fatalf("0.95 row selected %d, want 0", row.Selected)
		}
		assertAbsent(t, "Precision", row.Precision)
		assertMetric(t, "Recall", row.Recall, 0)
	})

	t.Run("no safe result leaves recall absent", func(t *testing.T) {
		r := compute(t, []Result{contested(t, 0.9)}, decision.DefaultThresholds())
		row := rowAt(t, r, 0.5)
		assertAbsent(t, "Recall", row.Recall)
		assertMetric(t, "Precision", row.Precision, 0)
	})
}

func TestOutcomesCountDecidedButUnsafe(t *testing.T) {
	r := compute(t, []Result{
		contested(t, 0.95),
		dominant(t, "a", 0.95),
		dominant(t, "a", 0.7),
		contested(t, 0.3),
	}, decision.DefaultThresholds())
	want := Counts{Decided: 2, Uncertain: 1, Escalate: 1, DecidedUnsafe: 1}
	if r.Outcomes != want {
		t.Fatalf("Outcomes = %+v, want %+v", r.Outcomes, want)
	}
}

func TestMetricMarshalsAbsentAsNull(t *testing.T) {
	b, err := json.Marshal(struct {
		A Metric `json:"a"`
		B Metric `json:"b"`
	}{A: metricOf(0.25)})
	if err != nil || string(b) != `{"a":0.25,"b":null}` {
		t.Fatalf("json = %s, %v; want {\"a\":0.25,\"b\":null}", b, err)
	}
}
