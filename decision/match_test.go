package decision

import (
	"reflect"
	"testing"
)

func TestMatchRunsExactlyTheHandlerForTheCase(t *testing.T) {
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
			var calls []string
			r := newResult(tt.kind, ship, mustProbability(t, 0.7), sampleProbabilities(t))

			got := Match(r,
				func(d Decided[action]) string {
					calls = append(calls, "decided")
					if d.Choice() != ship {
						t.Errorf("decided handler got choice %q, want %q", d.Choice(), ship)
					}
					return "decided"
				},
				func(u Uncertain[action]) string {
					calls = append(calls, "uncertain")
					if u.Leading() != ship {
						t.Errorf("uncertain handler got leading %q, want %q", u.Leading(), ship)
					}
					return "uncertain"
				},
				func(e Escalate[action]) string {
					calls = append(calls, "escalate")
					if e.Leading() != ship {
						t.Errorf("escalate handler got leading %q, want %q", e.Leading(), ship)
					}
					return "escalate"
				},
			)

			if got != tt.want || len(calls) != 1 || calls[0] != tt.want {
				t.Fatalf("Match returned %q after calling %v, want only %q", got, calls, tt.want)
			}
		})
	}
}

func TestMatchTreatsANilResultAsEscalate(t *testing.T) {
	var never Result[action]
	var gotLeading action = "unset"
	var gotConfidenceValid = true

	got := Match(never,
		func(Decided[action]) string { return "decided" },
		func(Uncertain[action]) string { return "uncertain" },
		func(e Escalate[action]) string {
			gotLeading, gotConfidenceValid = e.Leading(), e.Confidence().Valid()
			return "escalate"
		},
	)

	if got != "escalate" {
		t.Fatalf("Match(nil) ran the %q handler, want escalate", got)
	}
	if gotLeading != "" || gotConfidenceValid {
		t.Fatalf("the empty escalate value has leading %q and valid confidence %v, want neither",
			gotLeading, gotConfidenceValid)
	}
}

func TestMatchRequiresAHandlerForEachCase(t *testing.T) {
	typ := reflect.TypeOf(Match[action, string])

	if typ.NumIn() != 4 {
		t.Fatalf("Match takes %d parameters, want the result and three handlers", typ.NumIn())
	}
	for i := 1; i <= 3; i++ {
		if typ.In(i).Kind() != reflect.Func {
			t.Errorf("parameter %d of Match is %v, want a handler function", i, typ.In(i))
		}
	}
}
