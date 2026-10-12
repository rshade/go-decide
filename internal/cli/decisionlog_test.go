package cli

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rshade/go-decide/skills"
)

// decisionLogExample writes the jsonl example in the decide skill's
// decision-log reference to a file and returns its path.
func decisionLogExample(t *testing.T) string {
	t.Helper()
	doc, err := fs.ReadFile(skills.Decide(), "references/decision-log.md")
	if err != nil {
		t.Fatal(err)
	}
	_, rest, ok := strings.Cut(string(doc), "```jsonl\n")
	if !ok {
		t.Fatal("references/decision-log.md has no jsonl example")
	}
	block, _, ok := strings.Cut(rest, "```")
	if !ok {
		t.Fatal("the jsonl example in references/decision-log.md is not closed")
	}
	path := filepath.Join(t.TempDir(), "example.jsonl")
	if err := os.WriteFile(path, []byte(block), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// runOutputKey returns the first key, at any depth, that only a run produces.
func runOutputKey(v any) string {
	switch v := v.(type) {
	case map[string]any:
		for key, child := range v {
			switch key {
			case "pick", "choice", "leading", "confidence", "probabilities", "recommendation", "pre_screen", "path":
				return key
			}
			if found := runOutputKey(child); found != "" {
				return found
			}
		}
	case []any:
		for _, child := range v {
			if found := runOutputKey(child); found != "" {
				return found
			}
		}
	}
	return ""
}

func TestDecisionLogExampleReadsBackWithoutLeaks(t *testing.T) {
	lines, err := readLogLines(decisionLogExample(t))
	if err != nil {
		t.Fatalf("the store cannot read the example: %v", err)
	}
	kinds := map[string]int{}
	for _, l := range lines {
		kinds[l.Kind]++
		if l.Kind != kindDecision {
			continue
		}
		raw, err := json.Marshal(l)
		if err != nil {
			t.Fatal(err)
		}
		var whole any
		if err := json.Unmarshal(raw, &whole); err != nil {
			t.Fatal(err)
		}
		if key := runOutputKey(whole); key != "" {
			t.Errorf("decision %s holds run output under %q", l.ID, key)
		}
	}
	if kinds[kindDecision] == 0 || kinds[kindRun] == 0 || kinds[kindOutcome] == 0 {
		t.Errorf("example kinds = %v, want every kind shown", kinds)
	}
}

// TestDecisionLogExampleScoresWithEvalLog keeps the documented format and
// eval --log together: if either moves, eval rejects the example here.
func TestDecisionLogExampleScoresWithEvalLog(t *testing.T) {
	path := decisionLogExample(t)
	items, err := loadEvalLog(context.Background(), path, defaultEvalInstructions)
	if err != nil {
		t.Fatalf("eval --log rejects the example: %v", err)
	}
	lines, err := readLogLines(path)
	if err != nil {
		t.Fatal(err)
	}
	decisions := 0
	for _, l := range lines {
		if l.Kind == kindDecision {
			decisions++
		}
	}
	if len(items) == 0 || len(items) == decisions {
		t.Errorf("eval --log uses %d of %d decisions; the example should show a labelled and an unlabelled one", len(items), decisions)
	}
}
