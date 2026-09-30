package decision

import (
	"fmt"
	"math"
	"testing"
)

func TestClassify(t *testing.T) {
	th := DefaultThresholds()
	var unset Probability
	tests := []struct {
		name       string
		confidence Probability
		thresholds Thresholds
		want       kind
	}{
		{"zero", mustProbability(t, 0), th, kindEscalate},
		{"just below the floor", mustProbability(t, math.Nextafter(0.5, 0)), th, kindEscalate},
		{"exactly the floor", mustProbability(t, 0.5), th, kindUncertain},
		{"between the levels", mustProbability(t, 0.7), th, kindUncertain},
		{"just below the confident level", mustProbability(t, math.Nextafter(0.9, 0)), th, kindUncertain},
		{"exactly the confident level", mustProbability(t, 0.9), th, kindDecided},
		{"above the confident level", mustProbability(t, 0.95), th, kindDecided},
		{"one", mustProbability(t, 1), th, kindDecided},
		{"unset confidence", unset, th, kindEscalate},
		{"unset thresholds", mustProbability(t, 1), Thresholds{}, kindEscalate},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classify(tt.confidence, tt.thresholds); got != tt.want {
				t.Fatalf("classify(%v, %v/%v) = %v, want %v", tt.confidence.Float64(),
					tt.thresholds.Floor().Float64(), tt.thresholds.Confident().Float64(), got, tt.want)
			}
		})
	}
}

func TestNoValidThresholdsDecideBelowTheConfidentLevel(t *testing.T) {
	levels := []float64{0, 0.01, 0.25, 0.5, 0.58, 0.75, 0.9, 0.99, 1}
	for _, floor := range levels {
		for _, confident := range levels {
			th, err := NewThresholds(mustProbability(t, floor), mustProbability(t, confident))
			if err != nil {
				continue
			}
			for _, c := range levels {
				conf := mustProbability(t, c)
				got := classify(conf, th)
				t.Run(fmt.Sprintf("floor %v confident %v confidence %v", floor, confident, c), func(t *testing.T) {
					if conf.Less(th.Confident()) && got == kindDecided {
						t.Fatal("a confidence below the confident level was decided")
					}
					if conf.AtLeast(th.Confident()) && got != kindDecided {
						t.Fatalf("a confidence at or above the confident level was %v, want decided", got)
					}
				})
			}
		}
	}
}
