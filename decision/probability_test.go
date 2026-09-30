package decision

import (
	"errors"
	"math"
	"testing"
)

func TestNewProbability(t *testing.T) {
	tests := []struct {
		name    string
		in      float64
		wantErr bool
	}{
		{"zero", 0, false},
		{"one", 1, false},
		{"middle", 0.5, false},
		{"just above zero", math.SmallestNonzeroFloat64, false},
		{"just below one", math.Nextafter(1, 0), false},
		{"below zero", -0.01, true},
		{"above one", 1.01, true},
		{"just below zero", math.Nextafter(0, -1), true},
		{"just above one", math.Nextafter(1, 2), true},
		{"NaN", math.NaN(), true},
		{"positive infinity", math.Inf(1), true},
		{"negative infinity", math.Inf(-1), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewProbability(tt.in)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidProbability) {
					t.Fatalf("NewProbability(%v) error = %v, want ErrInvalidProbability", tt.in, err)
				}
				if p.Valid() {
					t.Fatalf("NewProbability(%v) returned a valid value alongside an error", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewProbability(%v) unexpected error: %v", tt.in, err)
			}
			if !p.Valid() || p.Float64() != tt.in {
				t.Fatalf("NewProbability(%v) = valid %v, %v; want valid, %v", tt.in, p.Valid(), p.Float64(), tt.in)
			}
		})
	}
}

func TestZeroProbabilityIsNotValid(t *testing.T) {
	var unset Probability
	zero, err := NewProbability(0)
	if err != nil {
		t.Fatalf("NewProbability(0) error: %v", err)
	}

	if unset.Valid() {
		t.Fatal("the zero value reports valid, want invalid")
	}
	if !zero.Valid() {
		t.Fatal("a probability created from 0 reports invalid, want valid")
	}
}

func TestProbabilityComparisons(t *testing.T) {
	p := func(f float64) Probability {
		t.Helper()
		v, err := NewProbability(f)
		if err != nil {
			t.Fatalf("NewProbability(%v) error: %v", f, err)
		}
		return v
	}
	var unset Probability
	tests := []struct {
		name         string
		a, b         Probability
		wantAtLeast  bool
		wantLessThan bool
	}{
		{"greater", p(0.9), p(0.5), true, false},
		{"equal", p(0.5), p(0.5), true, false},
		{"smaller", p(0.4), p(0.5), false, true},
		{"zero against zero", p(0), p(0), true, false},
		{"unset on the left", unset, p(0.5), false, false},
		{"unset on the right", p(0.5), unset, false, false},
		{"both unset", unset, unset, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.AtLeast(tt.b); got != tt.wantAtLeast {
				t.Errorf("AtLeast = %v, want %v", got, tt.wantAtLeast)
			}
			if got := tt.a.Less(tt.b); got != tt.wantLessThan {
				t.Errorf("Less = %v, want %v", got, tt.wantLessThan)
			}
		})
	}
}
