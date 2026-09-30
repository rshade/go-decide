package decision

import (
	"errors"
	"strings"
	"testing"
)

func TestFieldErrorNamesFieldAndKind(t *testing.T) {
	var err error = newFieldError(ErrInvalidQuestion, "State", "is empty")

	var fe *FieldError
	if !errors.As(err, &fe) || fe.Field != "State" || fe.Reason != "is empty" {
		t.Fatalf("errors.As = %+v, want field State and reason is empty", fe)
	}
	if !errors.Is(err, ErrInvalidQuestion) || errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("error %v should unwrap to ErrInvalidQuestion only", err)
	}
	if want := "decision: invalid question: State: is empty"; err.Error() != want {
		t.Fatalf("Error() = %q, want %q", err, want)
	}
}

func TestQuestionValidate(t *testing.T) {
	type nested struct{ A int }
	var nilPtr *nested
	var nilMap map[string]string
	var nilSlice []string
	tests := []struct {
		name      string
		state     any
		instr     string
		wantField string
		wantMsg   string
	}{
		{name: "string state", state: "the build passed"},
		{name: "object state", state: map[string]int{"tests": 412}},
		{name: "list state", state: []string{"a"}},
		{name: "no instructions", state: "x", instr: ""},
		{name: "nil", state: nil, wantField: "State", wantMsg: "empty"},
		{name: "typed nil pointer", state: nilPtr, wantField: "State", wantMsg: "empty"},
		{name: "nil map", state: nilMap, wantField: "State", wantMsg: "empty"},
		{name: "nil slice", state: nilSlice, wantField: "State", wantMsg: "empty"},
		{name: "empty string", state: "", wantField: "State", wantMsg: "empty"},
		{name: "empty map", state: map[string]int{}, wantField: "State", wantMsg: "empty"},
		{name: "empty list", state: []int{}, wantField: "State", wantMsg: "empty"},
		{name: "unencodable", state: make(chan int), wantField: "State", wantMsg: "encoded"},
		{name: "just under the limit", state: strings.Repeat("a", 4*maxStateTokens-200)},
		{name: "over the limit", state: strings.Repeat("a", 4*maxStateTokens+8), wantField: "State", wantMsg: "too large"},
		{name: "instructions count", state: strings.Repeat("a", 4*maxStateTokens-100),
			instr: strings.Repeat("b", 400), wantField: "State", wantMsg: "too large"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := sampleQuestion(t)
			q.State, q.Instructions = tt.state, tt.instr

			err := q.Validate()

			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			var fe *FieldError
			if !errors.As(err, &fe) || fe.Field != tt.wantField {
				t.Fatalf("Validate() = %v, want a FieldError on %s", err, tt.wantField)
			}
			if !errors.Is(err, ErrInvalidQuestion) || !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("Validate() = %q, want ErrInvalidQuestion mentioning %q", err, tt.wantMsg)
			}
		})
	}
}

func TestQuestionValidateRejectsZeroOptions(t *testing.T) {
	q := Question[action]{State: "x"}

	if err := q.Validate(); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("Validate() = %v, want ErrInvalidOptions", err)
	}
}
