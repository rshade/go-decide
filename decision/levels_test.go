package decision

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

type grade string

const (
	low  grade = "low"
	mid  grade = "medium"
	high grade = "high"
)

func sampleLevels(t *testing.T) Levels[grade] {
	t.Helper()
	l, err := NewLevels(Level[grade]{low, "minor"}, Level[grade]{mid, "noticeable"}, Level[grade]{Name: high})
	if err != nil {
		t.Fatalf("NewLevels error: %v", err)
	}
	return l
}

func numbered(n int) []Level[grade] {
	levels := make([]Level[grade], n)
	for i := range levels {
		levels[i] = Level[grade]{Name: grade(fmt.Sprintf("level-%d", i))}
	}
	return levels
}

func TestNewLevels(t *testing.T) {
	tests := []struct {
		name    string
		in      []Level[grade]
		wantMsg string
	}{
		{"two levels", numbered(2), ""},
		{"ten levels", numbered(MaxLevels), ""},
		{"none", nil, "at least two"},
		{"one", numbered(1), "at least two"},
		{"eleven", numbered(MaxLevels + 1), "at most 10"},
		{"empty name", []Level[grade]{{Name: low}, {Name: ""}}, "level 2 has an empty name"},
		{"duplicate", []Level[grade]{{Name: low}, {Name: mid}, {Name: low}}, `"low" is used by levels 1 and 3`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := NewLevels(tt.in...)

			if tt.wantMsg == "" {
				if err != nil || !l.Valid() || l.Len() != len(tt.in) {
					t.Fatalf("NewLevels = len %d, %v; want %d levels", l.Len(), err, len(tt.in))
				}
				return
			}
			var fe *FieldError
			if !errors.As(err, &fe) || fe.Field != "Levels" || !errors.Is(err, ErrInvalidLevels) {
				t.Fatalf("NewLevels error = %v, want a FieldError on Levels wrapping ErrInvalidLevels", err)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) || l.Valid() {
				t.Fatalf("NewLevels = valid %v, %q; want invalid and %q", l.Valid(), err, tt.wantMsg)
			}
		})
	}
}

func TestLevelsKeepOrderAndAreCopied(t *testing.T) {
	in := []Level[grade]{{low, "a"}, {mid, "b"}, {high, "c"}}
	l, err := NewLevels(in...)
	if err != nil {
		t.Fatalf("NewLevels error: %v", err)
	}

	in[0].Name = "changed"

	if got, want := l.Names(), []grade{low, mid, high}; !slices.Equal(got, want) {
		t.Errorf("Names() = %v, want %v", got, want)
	}
	if level, ok := l.At(1); !ok || level.Description != "b" {
		t.Errorf("At(1) = %v, %v; want the medium level", level, ok)
	}
	if _, ok := l.At(3); ok {
		t.Error("At(3) reported a level past the end")
	}
	if _, ok := l.At(-1); ok {
		t.Error("At(-1) reported a level")
	}
}

func TestZeroLevelsAreNotValid(t *testing.T) {
	var unset Levels[grade]

	if unset.Valid() || unset.Len() != 0 {
		t.Fatal("the zero Levels must be invalid and empty")
	}
}

func TestLevelCriteriaFallBackToTheName(t *testing.T) {
	got := sampleLevels(t).criteria()

	if want := []string{"minor", "noticeable", "high"}; !slices.Equal(got, want) {
		t.Fatalf("criteria() = %v, want %v", got, want)
	}
}
