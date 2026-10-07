package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rshade/ax-go/contract"

	"github.com/rshade/go-decide/eval"
)

const (
	bundledDecisions = "../../testdata/decisions.json"
	bundledTruth     = "../../testdata/decisions_truth.json"
)

const twoDecisions = `[
 {"id":"d1","title":"t","context":"c","constraints":["k"],
  "options":[{"name":"a","description":"A","pros":["cheap"]},{"name":"b","description":"B"}]},
 {"id":"d2","title":"t","context":"c","constraints":["k"],
  "options":[{"name":"a","description":"A"},{"name":"b","description":"B"}]}
]`

const twoTruths = `[
 {"id":"d1","class":"dominant","correct_option":"a","why":"w"},
 {"id":"d2","class":"contested","correct_option":null}
]`

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "f.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadEvalSetRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name      string
		decisions string
		truth     string
		wantField string
	}{
		{"empty decisions file", " ", twoTruths, "--decisions"},
		{"malformed truth", twoDecisions, `[{`, "--truth"},
		{"no decisions", `[]`, twoTruths, "--decisions"},
		{"empty id", strings.Replace(twoDecisions, `"id":"d1"`, `"id":""`, 1), twoTruths, "decisions[#0].id"},
		{"repeated id", strings.Replace(twoDecisions, `"id":"d2"`, `"id":"d1"`, 1), twoTruths, "decisions[d1].id"},
		{"decision without truth", twoDecisions, `[{"id":"d1","class":"dominant","correct_option":"a"}]`, "truth[d2].id"},
		{"truth without decision", twoDecisions, strings.Replace(twoTruths, `]`, `,{"id":"d3","class":"contested"}]`, 1), "decisions[d3].id"},
		{"unknown class", twoDecisions, strings.Replace(twoTruths, `"contested"`, `"split"`, 1), "truth[d2].class"},
		{"dominant without correct option", twoDecisions, strings.Replace(twoTruths, `"correct_option":"a"`, `"correct_option":null`, 1), "truth[d1].correct_option"},
		{"correct option not an option", twoDecisions, strings.Replace(twoTruths, `"correct_option":"a"`, `"correct_option":"Mainframe"`, 1), "truth[d1].correct_option"},
		{"contested with correct option", twoDecisions, strings.Replace(twoTruths, `"correct_option":null`, `"correct_option":"a"`, 1), "truth[d2].correct_option"},
		{"unknown truth field", twoDecisions, strings.Replace(twoTruths, `"why":"w"`, `"why":"w","confidence":1`, 1), "truth[d1]"},
		{"option without name", strings.Replace(twoDecisions, `"name":"b","description":"B"}]},`, `"description":"B"}]},`, 1), twoTruths, "decisions[d1].options[1].name"},
		{"repeated option name", strings.Replace(twoDecisions, `{"name":"b","description":"B"}]},`, `{"name":"a","description":"B"}]},`, 1), twoTruths, "decisions[d1].Options"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadEvalSet(context.Background(), writeFile(t, tt.decisions), writeFile(t, tt.truth), defaultEvalInstructions)
			var ce *contract.Error
			if !errors.As(err, &ce) {
				t.Fatalf("error = %v, want a validation envelope", err)
			}
			if ce.ErrorCode != "validation_error" || ce.Context["field"] != tt.wantField {
				t.Fatalf("error %s on field %v (%s), want validation_error on %s", ce.ErrorCode, ce.Context["field"], ce.Message, tt.wantField)
			}
		})
	}
}

func TestLoadEvalSetNamesTheUnknownField(t *testing.T) {
	decisions := strings.Replace(twoDecisions, `"title":"t"`, `"title":"t","priority":1`, 1)
	_, err := loadEvalSet(context.Background(), writeFile(t, decisions), writeFile(t, twoTruths), defaultEvalInstructions)
	var ce *contract.Error
	if !errors.As(err, &ce) || ce.Context["field"] != "decisions[d1]" || !strings.Contains(ce.Message, "priority") {
		t.Fatalf("error = %v, want a validation error on decisions[d1] naming priority", err)
	}
}

func TestLoadEvalSetKeepsOptionContentInTheState(t *testing.T) {
	items, err := loadEvalSet(context.Background(), writeFile(t, twoDecisions), writeFile(t, twoTruths), "Which fits?")
	if err != nil {
		t.Fatalf("loadEvalSet error: %v", err)
	}
	if len(items) != 2 || items[0].id != "d1" || items[0].class != eval.Dominant || items[0].correct != "a" || items[1].class != eval.Contested {
		t.Fatalf("items = %+v, want d1 dominant with a, then d2 contested", items)
	}
	state, err := json.Marshal(items[0].question.State)
	if err != nil || !strings.Contains(string(state), `"pros":["cheap"]`) {
		t.Fatalf("state = %s, %v; want the option's pros kept", state, err)
	}
	if items[0].question.Instructions != "Which fits?" {
		t.Fatalf("instructions = %q", items[0].question.Instructions)
	}
}

func TestLoadEvalSetReadsTheBundledFiles(t *testing.T) {
	items, err := loadEvalSet(context.Background(), bundledDecisions, bundledTruth, defaultEvalInstructions)
	if err != nil {
		t.Fatalf("loadEvalSet error: %v", err)
	}
	var dominant, contested int
	for _, it := range items {
		switch it.class {
		case eval.Dominant:
			dominant++
		case eval.Contested:
			contested++
		}
	}
	if len(items) != 40 || dominant != 20 || contested != 20 {
		t.Fatalf("%d items, %d dominant, %d contested; want 40, 20, 20", len(items), dominant, contested)
	}
}
