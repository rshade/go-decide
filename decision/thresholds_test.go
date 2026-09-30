package decision

import (
	"errors"
	"testing"
)

func mustProbability(t *testing.T, f float64) Probability {
	t.Helper()
	p, err := NewProbability(f)
	if err != nil {
		t.Fatalf("NewProbability(%v) error: %v", f, err)
	}
	return p
}

func TestNewThresholds(t *testing.T) {
	var unset Probability
	tests := []struct {
		name             string
		floor, confident Probability
		wantErr          bool
	}{
		{"valid", mustProbability(t, 0.4), mustProbability(t, 0.8), false},
		{"widest valid range", mustProbability(t, 0), mustProbability(t, 1), false},
		{"equal levels", mustProbability(t, 0.7), mustProbability(t, 0.7), true},
		{"floor above confident", mustProbability(t, 0.9), mustProbability(t, 0.5), true},
		{"unset floor", unset, mustProbability(t, 0.9), true},
		{"unset confident", mustProbability(t, 0.5), unset, true},
		{"both unset", unset, unset, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th, err := NewThresholds(tt.floor, tt.confident)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidThresholds) {
					t.Fatalf("NewThresholds error = %v, want ErrInvalidThresholds", err)
				}
				if th.Valid() {
					t.Fatal("NewThresholds returned a valid value alongside an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewThresholds unexpected error: %v", err)
			}
			if !th.Valid() || th.Floor() != tt.floor || th.Confident() != tt.confident {
				t.Fatalf("NewThresholds = floor %v confident %v valid %v", th.Floor(), th.Confident(), th.Valid())
			}
		})
	}
}

func TestZeroThresholdsAreNotValid(t *testing.T) {
	var unset Thresholds

	if unset.Valid() {
		t.Fatal("the zero Thresholds reports valid, want invalid")
	}
}

func TestDefaultThresholds(t *testing.T) {
	th := DefaultThresholds()

	if !th.Valid() {
		t.Fatal("DefaultThresholds() is not valid")
	}
	if got := th.Floor().Float64(); got != 0.5 {
		t.Errorf("default floor = %v, want 0.5", got)
	}
	if got := th.Confident().Float64(); got != 0.9 {
		t.Errorf("default confident level = %v, want 0.9", got)
	}
}

func mustThresholds(t *testing.T, floor, confident float64) Thresholds {
	t.Helper()
	th, err := NewThresholds(mustProbability(t, floor), mustProbability(t, confident))
	if err != nil {
		t.Fatalf("NewThresholds(%v, %v) error: %v", floor, confident, err)
	}
	return th
}
