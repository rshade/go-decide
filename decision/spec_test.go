package decision

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

func parse(t *testing.T, doc string) Spec {
	t.Helper()
	spec, err := ParseSpec(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("ParseSpec(%s) error: %v", doc, err)
	}
	return spec
}

func TestParseSpecReadsChoiceAndScoreInOrder(t *testing.T) {
	choice := parse(t, `{"state":"ready?","instructions":"decide","options":[{"name":"b","description":"second"},{"name":"a"}]}`)
	score := parse(t, `{"state":{"p":1},"levels":[{"name":"low"},{"name":"mid"},{"name":"high","description":"bad"}]}`)

	if len(choice.Options) != 2 || choice.Options[0].Name != "b" || choice.Options[1].Name != "a" || choice.Instructions != "decide" {
		t.Errorf("choice spec = %+v, want options b then a and instructions", choice)
	}
	if len(score.Levels) != 3 || score.Levels[2].Description != "bad" {
		t.Errorf("score spec = %+v, want three levels in order", score)
	}
}

func TestParseSpecRejectsBadDocuments(t *testing.T) {
	tests := []struct {
		name      string
		doc       string
		wantField string
	}{
		{"unknown field", `{"state":"x","color":"red"}`, "color"},
		{"malformed", `{"state":`, "Spec"},
		{"not json", `hello`, "Spec"},
		{"empty", "  \n", "Spec"},
		{"trailing content", `{"state":"x"} {"state":"y"}`, "Spec"},
		{"wrong type", `{"options":"ship"}`, "options"},
		{"wrong entry type", `{"options":[{"name":3}]}`, "options.0.name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSpec(strings.NewReader(tt.doc))

			var fe *FieldError
			if !errors.As(err, &fe) || fe.Field != tt.wantField || !errors.Is(err, ErrInvalidSpec) {
				t.Fatalf("ParseSpec(%s) error = %v, want a FieldError on %q wrapping ErrInvalidSpec", tt.doc, err, tt.wantField)
			}
		})
	}
}

func TestParseSpecBoundsWhatItReads(t *testing.T) {
	doc := `{"state":"` + strings.Repeat("a", maxSpecBytes) + `"}`

	_, err := ParseSpec(strings.NewReader(doc))

	if !errors.Is(err, ErrInvalidSpec) || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("ParseSpec(huge) error = %v, want a size rejection", err)
	}
}

func entries(names ...string) []SpecEntry {
	out := make([]SpecEntry, len(names))
	for i, n := range names {
		out[i] = SpecEntry{Name: n}
	}
	return out
}

func TestSpecQuestionValidates(t *testing.T) {
	many := make([]string, MaxOptions+1)
	for i := range many {
		many[i] = fmt.Sprint("o", i)
	}
	tests := []struct {
		name      string
		spec      Spec
		wantField string
		wantKind  error
		wantMsg   string
	}{
		{name: "valid", spec: Spec{State: "s", Options: entries("a", "b")}},
		{"duplicate names", Spec{State: "s", Options: entries("a", "b", "a")}, "Options", ErrInvalidOptions, `"a" is used by options 1 and 3`},
		{"empty name", Spec{State: "s", Options: entries("a", "")}, "Options", ErrInvalidOptions, "name is empty"},
		{"one option", Spec{State: "s", Options: entries("a")}, "Options", ErrInvalidOptions, "at least two"},
		{"no options", Spec{State: "s"}, "Options", ErrInvalidOptions, "at least two"},
		{"too many options", Spec{State: "s", Options: entries(many...)}, "Options", ErrInvalidOptions, "at most 255"},
		{"no state", Spec{Options: entries("a", "b")}, "State", ErrInvalidQuestion, "empty"},
		{"empty state", Spec{State: "", Options: entries("a", "b")}, "State", ErrInvalidQuestion, "empty"},
		{"oversized state", Spec{State: strings.Repeat("a", 4*maxStateTokens+8), Options: entries("a", "b")}, "State", ErrInvalidQuestion, "too large"},
		{"levels instead of options", Spec{State: "s", Levels: entries("a", "b")}, "Options", ErrInvalidOptions, "use score"},
		{"levels and options", Spec{State: "s", Options: entries("a", "b"), Levels: entries("x", "y")}, "Levels", ErrInvalidSpec, "not levels"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := tt.spec.Question()

			if tt.wantField == "" {
				if err != nil || !q.Options.Valid() || q.State != "s" {
					t.Fatalf("Question() = %+v, %v; want a valid question", q, err)
				}
				return
			}
			var fe *FieldError
			if !errors.As(err, &fe) || fe.Field != tt.wantField || !errors.Is(err, tt.wantKind) || !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("Question() error = %v, want a FieldError on %s wrapping %v mentioning %q", err, tt.wantField, tt.wantKind, tt.wantMsg)
			}
		})
	}
}

func TestSpecRateQuestionValidates(t *testing.T) {
	tests := []struct {
		name      string
		spec      Spec
		wantField string
		wantKind  error
		wantMsg   string
	}{
		{name: "valid", spec: Spec{State: "s", Levels: entries("a", "b", "c")}},
		{"duplicate names", Spec{State: "s", Levels: entries("a", "b", "a")}, "Levels", ErrInvalidLevels, `"a" is used by levels 1 and 3`},
		{"one level", Spec{State: "s", Levels: entries("a")}, "Levels", ErrInvalidLevels, "at least two"},
		{"eleven levels", Spec{State: "s", Levels: entries(strings.Split("a b c d e f g h i j k", " ")...)}, "Levels", ErrInvalidLevels, "at most 10"},
		{"no state", Spec{Levels: entries("a", "b")}, "State", ErrInvalidQuestion, "empty"},
		{"options instead of levels", Spec{State: "s", Options: entries("a", "b")}, "Levels", ErrInvalidLevels, "use ask"},
		{"levels and options", Spec{State: "s", Options: entries("a", "b"), Levels: entries("x", "y")}, "Options", ErrInvalidSpec, "not options"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := tt.spec.RateQuestion()

			if tt.wantField == "" {
				if err != nil || !q.Levels.Valid() {
					t.Fatalf("RateQuestion() = %+v, %v; want a valid question", q, err)
				}
				return
			}
			var fe *FieldError
			if !errors.As(err, &fe) || fe.Field != tt.wantField || !errors.Is(err, tt.wantKind) || !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("RateQuestion() error = %v, want a FieldError on %s wrapping %v mentioning %q", err, tt.wantField, tt.wantKind, tt.wantMsg)
			}
		})
	}
}

func TestDecisionFixturesLoadAsSpecs(t *testing.T) {
	raw, err := os.ReadFile("../testdata/decisions.json")
	if err != nil {
		t.Fatalf("reading the fixtures: %v", err)
	}
	var fixtures []struct {
		ID          string      `json:"id"`
		Title       string      `json:"title"`
		Context     string      `json:"context"`
		Constraints []string    `json:"constraints"`
		Options     []SpecEntry `json:"options"`
	}
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatalf("decoding the fixtures: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no fixtures to check")
	}

	for _, f := range fixtures {
		doc, err := json.Marshal(Spec{
			State:        map[string]any{"context": f.Context, "constraints": f.Constraints},
			Instructions: f.Title,
			Options:      f.Options,
		})
		if err != nil {
			t.Fatalf("%s: encoding: %v", f.ID, err)
		}
		q, err := parse(t, string(doc)).Question()
		if err != nil || q.Options.Len() != len(f.Options) {
			t.Errorf("%s: Question() = %d options, %v; want %d options and no error", f.ID, q.Options.Len(), err, len(f.Options))
		}
	}
}

func TestMergeFlags(t *testing.T) {
	doc := Spec{State: "doc", Options: entries("a", "b")}

	t.Run("flags fill missing fields", func(t *testing.T) {
		got, err := Spec{Options: entries("a", "b")}.MergeFlags(Spec{State: "flag", Instructions: "go"})

		if err != nil || got.State != "flag" || got.Instructions != "go" || len(got.Options) != 2 {
			t.Fatalf("MergeFlags = %+v, %v; want the document options with the flag state and instructions", got, err)
		}
	})
	t.Run("flags alone", func(t *testing.T) {
		got, err := Spec{}.MergeFlags(Spec{State: "s", Levels: entries("x", "y")})

		if err != nil || got.State != "s" || len(got.Levels) != 2 {
			t.Fatalf("MergeFlags = %+v, %v; want the flag values", got, err)
		}
	})
	for _, tt := range []struct {
		name  string
		flags Spec
		field string
	}{
		{"state", Spec{State: "flag"}, "State"},
		{"instructions", Spec{Instructions: "flag"}, "Instructions"},
		{"options", Spec{Options: entries("c", "d")}, "Options"},
	} {
		t.Run("conflict on "+tt.name, func(t *testing.T) {
			base := doc
			base.Instructions = "doc"

			_, err := base.MergeFlags(tt.flags)

			var fe *FieldError
			if !errors.As(err, &fe) || fe.Field != tt.field || !errors.Is(err, ErrInvalidSpec) {
				t.Fatalf("MergeFlags error = %v, want a conflict on %s", err, tt.field)
			}
		})
	}
	t.Run("conflict on levels", func(t *testing.T) {
		_, err := Spec{Levels: entries("a", "b")}.MergeFlags(Spec{Levels: entries("c", "d")})

		var fe *FieldError
		if !errors.As(err, &fe) || fe.Field != "Levels" {
			t.Fatalf("MergeFlags error = %v, want a conflict on Levels", err)
		}
	})
}
