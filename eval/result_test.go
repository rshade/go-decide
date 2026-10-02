package eval

import (
	"errors"
	"testing"

	"github.com/rshade/jev-decide/decision"
)

func prob(t *testing.T, f float64) decision.Probability {
	t.Helper()
	p, err := decision.NewProbability(f)
	if err != nil {
		t.Fatalf("NewProbability(%v) error: %v", f, err)
	}
	return p
}

func TestNewResultRejectsInvalidInput(t *testing.T) {
	var unset decision.Probability
	tests := []struct {
		name       string
		class      Class
		pick       string
		confidence decision.Probability
		correct    string
	}{
		{"class not set", 0, "a", prob(t, 0.9), "a"},
		{"confidence not set", Dominant, "a", unset, "a"},
		{"empty pick", Dominant, "", prob(t, 0.9), "a"},
		{"dominant without correct option", Dominant, "a", prob(t, 0.9), ""},
		{"contested with correct option", Contested, "a", prob(t, 0.9), "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := NewResult(tt.class, tt.pick, tt.confidence, tt.correct)
			if !errors.Is(err, ErrInvalidResult) {
				t.Fatalf("NewResult error = %v, want ErrInvalidResult", err)
			}
			if r != (Result{}) {
				t.Fatalf("NewResult returned %+v alongside an error", r)
			}
		})
	}
}

func TestSafe(t *testing.T) {
	tests := []struct {
		name    string
		class   Class
		pick    string
		correct string
		want    bool
	}{
		{"correct pick on dominant", Dominant, "a", "a", true},
		{"wrong pick on dominant", Dominant, "b", "a", false},
		{"contested at 0.99", Contested, "a", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := NewResult(tt.class, tt.pick, prob(t, 0.99), tt.correct)
			if err != nil {
				t.Fatalf("NewResult error: %v", err)
			}
			if got := r.Safe(); got != tt.want {
				t.Fatalf("Safe() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseClass(t *testing.T) {
	for _, s := range []string{"dominant", "contested"} {
		c, err := ParseClass(s)
		if err != nil || c.String() != s {
			t.Errorf("ParseClass(%q) = %v, %v; want %s", s, c, err, s)
		}
	}
	if _, err := ParseClass("Dominant"); !errors.Is(err, ErrInvalidResult) {
		t.Errorf("ParseClass(%q) error = %v, want ErrInvalidResult", "Dominant", err)
	}
}

func TestOutcomeOfMatchesTheChoiceRuleAtTheBoundaries(t *testing.T) {
	th := decision.DefaultThresholds()
	tests := []struct {
		confidence float64
		want       Outcome
	}{
		{1, Decided},
		{0.9, Decided},
		{0.8999, Uncertain},
		{0.5, Uncertain},
		{0.4999, Escalate},
		{0, Escalate},
	}
	for _, tt := range tests {
		if got := OutcomeOf(prob(t, tt.confidence), th); got != tt.want {
			t.Errorf("OutcomeOf(%v) = %v, want %v", tt.confidence, got, tt.want)
		}
	}
	var unset decision.Probability
	if got := OutcomeOf(unset, th); got != Escalate {
		t.Errorf("OutcomeOf(unset) = %v, want escalate", got)
	}
	if got := OutcomeOf(prob(t, 1), decision.Thresholds{}); got != Escalate {
		t.Errorf("OutcomeOf with unset thresholds = %v, want escalate", got)
	}
}
