package decision

import (
	"fmt"
	"testing"
)

func sampleProbabilities(t *testing.T) map[action]Probability {
	t.Helper()
	return map[action]Probability{
		ship: mustProbability(t, 0.7),
		hold: mustProbability(t, 0.2),
		kill: mustProbability(t, 0.1),
	}
}

func classOf(r Result[action]) string {
	switch r.(type) {
	case Decided[action]:
		return "decided"
	case Uncertain[action]:
		return "uncertain"
	case Escalate[action]:
		return "escalate"
	default:
		return "unknown"
	}
}

func TestTypeSwitchTellsTheThreeCasesApart(t *testing.T) {
	tests := []struct {
		kind kind
		want string
	}{
		{kindDecided, "decided"},
		{kindUncertain, "uncertain"},
		{kindEscalate, "escalate"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			r := newResult(tt.kind, ship, mustProbability(t, 0.7), sampleProbabilities(t))

			if got := classOf(r); got != tt.want {
				t.Fatalf("type switch on a %v result = %q, want %q", tt.kind, got, tt.want)
			}
		})
	}
}

func TestResultsExposeWhatTheyWereBuiltWith(t *testing.T) {
	for _, k := range []kind{kindDecided, kindUncertain, kindEscalate} {
		t.Run(k.String(), func(t *testing.T) {
			probs := sampleProbabilities(t)
			r := newResult(k, ship, mustProbability(t, 0.7), probs)

			if r.Leading() != ship {
				t.Errorf("Leading() = %q, want %q", r.Leading(), ship)
			}
			if got := r.Confidence().Float64(); got != 0.7 {
				t.Errorf("Confidence() = %v, want 0.7", got)
			}
			if got := r.Probabilities(); len(got) != 3 || got[hold] != probs[hold] {
				t.Errorf("Probabilities() = %v, want %v", got, probs)
			}
		})
	}
}

func TestOnlyDecidedHasAChoice(t *testing.T) {
	type chooser interface{ Choice() action }
	tests := []struct {
		kind       kind
		wantChoice bool
	}{
		{kindDecided, true},
		{kindUncertain, false},
		{kindEscalate, false},
	}
	for _, tt := range tests {
		t.Run(tt.kind.String(), func(t *testing.T) {
			r := newResult(tt.kind, ship, mustProbability(t, 0.7), sampleProbabilities(t))

			c, ok := r.(chooser)
			if ok != tt.wantChoice {
				t.Fatalf("a %v result has Choice() = %v, want %v", tt.kind, ok, tt.wantChoice)
			}
			if ok && c.Choice() != ship {
				t.Fatalf("Choice() = %q, want %q", c.Choice(), ship)
			}
		})
	}
}

func TestProbabilitiesAreCopiedBothWays(t *testing.T) {
	probs := sampleProbabilities(t)
	r := newResult(kindDecided, ship, mustProbability(t, 0.7), probs)

	probs[ship] = mustProbability(t, 0.01)
	got := r.Probabilities()
	got[hold] = mustProbability(t, 0.99)

	if r.Probabilities()[ship].Float64() != 0.7 {
		t.Error("the result followed a change to the map it was built from")
	}
	if r.Probabilities()[hold].Float64() != 0.2 {
		t.Error("the result followed a change to the map it returned")
	}
}

func TestResultStringNamesTheCaseAndTheLeadingOption(t *testing.T) {
	tests := []struct {
		kind kind
		want string
	}{
		{kindDecided, "decided choice=ship confidence=0.70"},
		{kindUncertain, "uncertain leading=ship confidence=0.70"},
		{kindEscalate, "escalate leading=ship confidence=0.70"},
	}
	for _, tt := range tests {
		t.Run(tt.kind.String(), func(t *testing.T) {
			r := newResult(tt.kind, ship, mustProbability(t, 0.7), sampleProbabilities(t))

			if got := fmt.Sprint(r); got != tt.want {
				t.Fatalf("fmt.Sprint = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEmptyEscalateStringShowsUnsetConfidence(t *testing.T) {
	got := Match[action, string](nil,
		func(d Decided[action]) string { return fmt.Sprint(d) },
		func(u Uncertain[action]) string { return fmt.Sprint(u) },
		func(e Escalate[action]) string { return fmt.Sprint(e) },
	)

	if want := "escalate leading= confidence=unset"; got != want {
		t.Fatalf("fmt.Sprint = %q, want %q", got, want)
	}
}
