package cli

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/rshade/go-decide/eval"
)

func outcomeFixture(id, class, correct string, at time.Time) string {
	option := "null"
	if correct != "" {
		option = `"` + correct + `"`
	}
	return `{"v":1,"kind":"outcome","id":"` + id + `","recorded_at":"` + at.Format(time.RFC3339) + `","trace_id":"t",` +
		`"truth":{"id":"` + id + `","class":"` + class + `","correct_option":` + option + `,"why":"w"}}`
}

func TestEvalLogUsesOnlyLabelledDecisions(t *testing.T) {
	path := writeLogFixture(t,
		decisionFixture("2026-09-01-aaaa", fixedNow.AddDate(0, 0, -40)),
		decisionFixture("2026-09-02-bbbb", fixedNow.AddDate(0, 0, -39)),
		outcomeFixture("2026-09-01-aaaa", "dominant", "a", fixedNow),
	)
	r := execute(t, "", []string{"eval", "--dry-run", "--log", path}, http.StatusOK, "")
	out := decode[EvalDryRunOutput](t, r)
	if r.code != 0 || out.Decisions != 1 || out.Dominant != 1 || r.hits != 0 {
		t.Errorf("exit %d output %+v hits %d, want 0 with one dominant decision and no request; stderr %s", r.code, out, r.hits, r.stderr)
	}
}

func TestEvalLogTakesTheLatestOutcome(t *testing.T) {
	id := "2026-09-01-aaaa"
	path := writeLogFixture(t,
		decisionFixture(id, fixedNow.AddDate(0, 0, -40)),
		outcomeFixture(id, "dominant", "a", fixedNow.AddDate(0, 0, -5)),
		outcomeFixture(id, "contested", "", fixedNow.AddDate(0, 0, -5)),
	)
	items, err := loadEvalLog(context.Background(), path, defaultEvalInstructions)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].class != eval.Contested {
		t.Errorf("items = %+v, want one contested decision from the later outcome line", items)
	}
}

func TestEvalLogRefusesNothingLabelledAndMixedInputs(t *testing.T) {
	unlabelled := writeLogFixture(t, decisionFixture("2026-09-01-aaaa", fixedNow))
	for name, args := range map[string][]string{
		"nothing labelled": {"eval", "--dry-run", "--log", unlabelled},
		"with decisions":   {"eval", "--dry-run", "--log", unlabelled, "--decisions", "testdata/eval/decisions.json"},
		"with truth":       {"eval", "--dry-run", "--log", unlabelled, "--truth", "testdata/eval/truth.json"},
	} {
		t.Run(name, func(t *testing.T) {
			r := execute(t, "", args, http.StatusOK, "")
			failsNaming(t, r, "--log", unlabelled, 1)
			if r.hits != 0 {
				t.Errorf("sent %d requests, want 0", r.hits)
			}
		})
	}
}
