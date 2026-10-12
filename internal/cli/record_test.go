package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runRecord runs args with DECIDE_LOG set to logPath ("" for unset), a fixed
// clock and a seeded id source.
func runRecord(t *testing.T, logPath string, args ...string) run {
	t.Helper()
	return runRecordEnv(t, map[string]string{decideLogEnv: logPath}, args...)
}

// runRecordEnv is runRecord with vars as the whole environment.
func runRecordEnv(t *testing.T, vars map[string]string, args ...string) run {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), args, Env{
		Stdin:   strings.NewReader(""),
		Stdout:  &stdout,
		Stderr:  &stderr,
		Getenv:  func(key string) string { return vars[key] },
		Version: goldenVersion,
		Now:     func() time.Time { return fixedNow },
		Rand:    rand.NewChaCha8([32]byte{7}),
	})
	return run{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

var recordDecisionFlags = []string{
	"record", "decision",
	"--title", "Cache backend for the pricing service",
	"--context", "Reads dominate, and the service runs next to a managed Postgres.",
	"--constraint", "No new service to operate",
	"--instructions", "Which cache should the service use?",
	"--option", "postgres=Cache in an unlogged Postgres table",
	"--option", "sqlite=Cache in a local SQLite file",
}

const decidedPreScreen = `{"schema_version":7,"backend":"jev","outcome":"decided","choice":"postgres",` +
	`"confidence":0.95,"probabilities":{"postgres":0.95,"sqlite":0.05},"thresholds":{"floor":0.5,"confident":0.9}}`

const escalatePreScreen = `{"schema_version":7,"backend":"jev","outcome":"escalate","leading":"postgres",` +
	`"confidence":0.41,"probabilities":{"postgres":0.52,"sqlite":0.48},"thresholds":{"floor":0.5,"confident":0.9}}`

const citation = `{"agent":"advocate_b","round":2,"backend":"jev","outcome":"decided","result":"sqlite",` +
	`"confidence":0.95,"asked":"Which cache keeps latency lowest?","options":[{"name":"sqlite","probability":0.95}]}`

// newLog records one decision in a fresh log and returns the log and its id.
func newLog(t *testing.T) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "decisions.jsonl")
	r := runRecord(t, path, recordDecisionFlags...)
	if r.code != 0 {
		t.Fatalf("record decision exit %d: %s", r.code, r.stderr)
	}
	return path, decode[RecordOutput](t, r).ID
}

func logLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	return readLog(t, path)
}

// failsNaming checks r exited 2 with a validation error whose field is field,
// and that the log at path still has want lines.
func failsNaming(t *testing.T, r run, field, path string, want int) {
	t.Helper()
	var e struct {
		Context struct {
			Field string `json:"field"`
		} `json:"context"`
	}
	_ = json.Unmarshal([]byte(r.stderr), &e)
	if r.code != 2 || e.Context.Field != field {
		t.Errorf("exit %d field %q, want 2 naming %q; stderr %s", r.code, e.Context.Field, field, r.stderr)
	}
	if got := len(logLines(t, path)); got != want {
		t.Errorf("log has %d lines after a refused record, want %d", got, want)
	}
}

func TestRecordDecisionWritesEvalsShape(t *testing.T) {
	path, id := newLog(t)
	lines := logLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("log has %d lines, want 1", len(lines))
	}
	got := lines[0]
	if got["kind"] != "decision" || got["id"] != id || got["instructions"] != "Which cache should the service use?" {
		t.Errorf("line = %v", got)
	}
	d, _ := got["decision"].(map[string]any)
	keys := make([]string, 0, len(d))
	for k := range d {
		keys = append(keys, k)
	}
	if len(d) != 5 || d["id"] != id || d["title"] == nil || d["context"] == nil || d["constraints"] == nil || d["options"] == nil {
		t.Errorf("decision keys %v, want exactly id, title, context, constraints and options", keys)
	}
	for _, leak := range []string{"pick", "choice", "leading", "confidence", "probabilities", "recommendation", "path"} {
		if _, ok := got[leak]; ok {
			t.Errorf("decision line holds %q", leak)
		}
	}
}

func TestRecordDecisionReportsIDAndLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "decisions.jsonl")
	out := decode[RecordOutput](t, runRecord(t, path, recordDecisionFlags...))
	if !out.Recording || out.Kind != "decision" || out.Log != path || !strings.HasPrefix(out.ID, "2026-10-12-") || out.SchemaVersion != SchemaVersion {
		t.Errorf("output = %+v", out)
	}
}

func TestRecordDecisionRefusesAnInvalidDecision(t *testing.T) {
	tests := map[string]struct {
		field string
		args  []string
	}{
		"one option":      {"option", []string{"--title", "t", "--context", "c", "--instructions", "Which?", "--option", "a=x"}},
		"repeated name":   {"option", []string{"--title", "t", "--context", "c", "--instructions", "Which?", "--option", "a=x", "--option", "a=y"}},
		"no instructions": {"instructions", []string{"--title", "t", "--context", "c", "--option", "a=x", "--option", "b=y"}},
		"no title":        {"title", []string{"--context", "c", "--instructions", "Which?", "--option", "a=x", "--option", "b=y"}},
		"unnamed option":  {"option", []string{"--title", "t", "--context", "c", "--instructions", "Which?", "--option", "=x", "--option", "b=y"}},
		"oversized context": {"context", []string{"--title", "t", "--context", strings.Repeat("x", 140_000), "--instructions", "Which?",
			"--option", "a=x", "--option", "b=y"}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "decisions.jsonl")
			failsNaming(t, runRecord(t, path, append([]string{"record", "decision"}, tt.args...)...), tt.field, path, 0)
			if _, err := os.Stat(path); err == nil {
				t.Error("a refused decision created the log")
			}
		})
	}
}

func TestRecordIsOffWithoutDecideLog(t *testing.T) {
	for _, args := range [][]string{
		recordDecisionFlags,
		{"record", "run", "--id", "x", "--path", "fast", "--pre-screen", decidedPreScreen},
		{"record", "outcome", "--id", "x", "--class", "contested", "--why", "w"},
	} {
		for _, dryRun := range []bool{false, true} {
			full := args
			if dryRun {
				full = append([]string{"--dry-run"}, args...)
			}
			r := runRecord(t, "", full...)
			out := decode[RecordOutput](t, r)
			if r.code != 0 || out.Recording || out.DryRun != dryRun || out.ID != "" || out.Log != "" {
				t.Errorf("%v: exit %d output %+v, want 0 with recording off and dry_run %v", full, r.code, out, dryRun)
			}
		}
	}
}

func TestRecordRunRefusesBadInput(t *testing.T) {
	path, id := newLog(t)
	tests := map[string]struct {
		field string
		args  []string
	}{
		"unknown id":             {"id", []string{"--id", "2026-01-01-ffff", "--path", "fast", "--pre-screen", decidedPreScreen}},
		"unknown path":           {"path", []string{"--id", id, "--path", "maybe", "--pre-screen", decidedPreScreen}},
		"fast but escalate":      {"path", []string{"--id", id, "--path", "fast", "--pre-screen", escalatePreScreen}},
		"two sources":            {"pre-screen", []string{"--id", id, "--path", "stopped", "--pre-screen", escalatePreScreen, "--pre-screen-error", "jev.timeout"}},
		"no source":              {"pre-screen", []string{"--id", id, "--path", "stopped"}},
		"unknown envelope key":   {"pre-screen", []string{"--id", id, "--path", "fast", "--pre-screen", strings.Replace(decidedPreScreen, `"backend"`, `"verdict":"go","backend"`, 1)}},
		"envelope no confidence": {"pre-screen", []string{"--id", id, "--path", "fast", "--pre-screen", strings.Replace(decidedPreScreen, `"confidence":0.95,`, "", 1)}},
		"envelope no backend":    {"pre-screen", []string{"--id", id, "--path", "fast", "--pre-screen", strings.Replace(decidedPreScreen, `"backend":"jev",`, "", 1)}},
		"envelope null probabilities": {"pre-screen", []string{"--id", id, "--path", "fast", "--pre-screen",
			strings.Replace(decidedPreScreen, `{"postgres":0.95,"sqlite":0.05}`, "null", 1)}},
		"envelope empty probabilities": {"pre-screen", []string{"--id", id, "--path", "fast", "--pre-screen",
			strings.Replace(decidedPreScreen, `{"postgres":0.95,"sqlite":0.05}`, "{}", 1)}},
		"envelope no floor":       {"pre-screen", []string{"--id", id, "--path", "fast", "--pre-screen", strings.Replace(decidedPreScreen, `"floor":0.5,`, "", 1)}},
		"envelope bare":           {"pre-screen", []string{"--id", id, "--path", "fast", "--pre-screen", `{"outcome":"decided","choice":"postgres"}`}},
		"citation no result":      {"citation", []string{"--id", id, "--path", "debate", "--pre-screen", escalatePreScreen, "--recommendation", "r", "--citation", strings.Replace(citation, `"result":"sqlite",`, "", 1)}},
		"citation no confidence":  {"citation", []string{"--id", id, "--path", "debate", "--pre-screen", escalatePreScreen, "--recommendation", "r", "--citation", strings.Replace(citation, `"confidence":0.95,`, "", 1)}},
		"citation no probability": {"citation", []string{"--id", id, "--path", "debate", "--pre-screen", escalatePreScreen, "--recommendation", "r", "--citation", strings.Replace(citation, `,"probability":0.95`, "", 1)}},
		"debate fields":           {"recommendation", []string{"--id", id, "--path", "stopped", "--pre-screen-error", "jev.timeout", "--recommendation", "r"}},
		"citation unknown key":    {"citation", []string{"--id", id, "--path", "debate", "--pre-screen", escalatePreScreen, "--recommendation", "r", "--citation", strings.Replace(citation, `"round":2`, `"round":2,"note":"x"`, 1)}},
		"citation no asked":       {"citation", []string{"--id", id, "--path", "debate", "--pre-screen", escalatePreScreen, "--recommendation", "r", "--citation", strings.Replace(citation, `"asked":"Which cache keeps latency lowest?"`, `"asked":""`, 1)}},
		"debate no reco":          {"recommendation", []string{"--id", id, "--path", "debate", "--pre-screen", escalatePreScreen}},
		"bad evidence mode":       {"evidence-mode", []string{"--id", id, "--path", "debate", "--pre-screen", escalatePreScreen, "--recommendation", "r", "--evidence-mode", "peeking"}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			failsNaming(t, runRecord(t, path, append([]string{"record", "run"}, tt.args...)...), tt.field, path, 1)
		})
	}
}

func TestRecordRunKeepsTheDebateEvidence(t *testing.T) {
	path, id := newLog(t)
	r := runRecord(t, path, "record", "run", "--id", id, "--path", "debate", "--pre-screen", escalatePreScreen,
		"--recommendation", "Use Postgres.", "--evidence-mode", "blind", "--go-decide-calls", "1", "--citation", citation)
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	run := logLines(t, path)[1]
	debate, _ := run["debate"].(map[string]any)
	citations, _ := debate["citations"].([]any)
	if run["path"] != "debate" || debate["evidence_mode"] != "blind" || debate["go_decide_calls"] != float64(1) || len(citations) != 1 {
		t.Fatalf("run line = %v", run)
	}
	c, _ := citations[0].(map[string]any)
	if c["agent"] != "advocate_b" || c["round"] != float64(2) || c["asked"] != "Which cache keeps latency lowest?" {
		t.Errorf("citation = %v", c)
	}
}

func TestRecordRunAppendsASecondRun(t *testing.T) {
	path, id := newLog(t)
	if r := runRecord(t, path, "record", "run", "--id", id, "--path", "fast", "--pre-screen", decidedPreScreen); r.code != 0 {
		t.Fatalf("fast run exit %d: %s", r.code, r.stderr)
	}
	first, _ := os.ReadFile(path)
	if r := runRecord(t, path, "record", "run", "--id", id, "--path", "debate", "--pre-screen", decidedPreScreen, "--recommendation", "r"); r.code != 0 {
		t.Fatalf("debate run exit %d: %s", r.code, r.stderr)
	}
	second, _ := os.ReadFile(path)
	if !bytes.HasPrefix(second, first) || len(logLines(t, path)) != 3 {
		t.Errorf("second run rewrote the log or did not append")
	}
}

func TestRecordRunAFailureHasNoOutcome(t *testing.T) {
	path, id := newLog(t)
	if r := runRecord(t, path, "record", "run", "--id", id, "--path", "stopped", "--pre-screen-error", "jev.timeout"); r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	pre, _ := logLines(t, path)[1]["pre_screen"].(map[string]any)
	if len(pre) != 1 || pre["error"] != "jev.timeout" {
		t.Errorf("pre_screen = %v, want only the error code", pre)
	}
}

func TestRecordOutcomeChecksTheDecisionsOptions(t *testing.T) {
	path, id := newLog(t)
	r := runRecord(t, path, "record", "outcome", "--id", id, "--class", "dominant", "--correct-option", "redis", "--why", "w")
	failsNaming(t, r, "correct-option", path, 1)

	if r := runRecord(t, path, "record", "outcome", "--id", id, "--class", "contested", "--why", "Still split."); r.code != 0 {
		t.Fatalf("contested exit %d: %s", r.code, r.stderr)
	}
	truth, _ := logLines(t, path)[1]["truth"].(map[string]any)
	if v, ok := truth["correct_option"]; !ok || v != nil || truth["class"] != "contested" || truth["id"] != id {
		t.Errorf("truth = %v, want contested with correct_option null", truth)
	}

	if r := runRecord(t, path, "record", "outcome", "--id", id, "--class", "dominant", "--correct-option", "postgres", "--why", "Held up."); r.code != 0 {
		t.Fatalf("relabel exit %d: %s", r.code, r.stderr)
	}
	if got := len(logLines(t, path)); got != 3 {
		t.Errorf("log has %d lines after a relabel, want 3", got)
	}
}

func TestRecordOutcomeRefusesBadInput(t *testing.T) {
	path, id := newLog(t)
	for name, tt := range map[string]struct {
		field string
		args  []string
	}{
		"unknown id":           {"id", []string{"--id", "2026-01-01-ffff", "--class", "contested", "--why", "w"}},
		"unknown class":        {"class", []string{"--id", id, "--class", "obvious", "--why", "w"}},
		"contested with right": {"correct-option", []string{"--id", id, "--class", "contested", "--correct-option", "postgres", "--why", "w"}},
		"no why":               {"why", []string{"--id", id, "--class", "contested"}},
	} {
		t.Run(name, func(t *testing.T) {
			failsNaming(t, runRecord(t, path, append([]string{"record", "outcome"}, tt.args...)...), tt.field, path, 1)
		})
	}
}

func writeLogFixture(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "decisions.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func decisionFixture(id string, at time.Time) string {
	return `{"v":1,"kind":"decision","id":"` + id + `","recorded_at":"` + at.Format(time.RFC3339) + `","trace_id":"t",` +
		`"instructions":"Which?","decision":{"id":"` + id + `","title":"t","context":"c","constraints":[],` +
		`"options":[{"name":"a","description":"x"},{"name":"b","description":"y"}]}}`
}

func TestRecordPendingListsOldUnlabelledDecisionsWithoutRuns(t *testing.T) {
	old, recent, labelled := "2026-09-02-aaaa", "2026-10-02-bbbb", "2026-08-01-cccc"
	path := writeLogFixture(t,
		decisionFixture(old, fixedNow.AddDate(0, 0, -40)),
		`{"v":1,"kind":"run","id":"`+old+`","recorded_at":"2026-09-02T15:00:00Z","trace_id":"t","path":"fast","pre_screen":`+decidedPreScreen+`}`,
		decisionFixture(recent, fixedNow.AddDate(0, 0, -10)),
		decisionFixture(labelled, fixedNow.AddDate(0, 0, -70)),
		`{"v":1,"kind":"outcome","id":"`+labelled+`","recorded_at":"2026-10-01T00:00:00Z","trace_id":"t","truth":{"id":"`+labelled+`","class":"contested","correct_option":null,"why":"w"}}`,
	)
	r := runRecord(t, path, "record", "pending")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	out := decode[PendingOutput](t, r)
	if !out.Recording || out.OlderThanDays != 30 || len(out.Decisions) != 1 || out.Decisions[0].ID != old {
		t.Fatalf("output = %+v, want only %s", out, old)
	}
	for _, leak := range []string{"pre_screen", "path", "choice", "confidence", "probabilities", "recommendation", "postgres"} {
		if strings.Contains(r.stdout, leak) {
			t.Errorf("pending output holds %q: %s", leak, r.stdout)
		}
	}

	if out := decode[PendingOutput](t, runRecord(t, path, "record", "pending", "--older-than-days", "0")); len(out.Decisions) != 2 {
		t.Errorf("threshold 0 lists %d decisions, want 2", len(out.Decisions))
	}
}

func TestRecordPendingIsOffWithoutDecideLog(t *testing.T) {
	r := runRecord(t, "", "record", "pending")
	out := decode[PendingOutput](t, r)
	if r.code != 0 || out.Recording || out.Decisions == nil || len(out.Decisions) != 0 {
		t.Errorf("exit %d output %+v, want 0, recording off and an empty list", r.code, out)
	}
}

func TestRecordDryRunValidatesAndWritesNothing(t *testing.T) {
	inRepo := filepath.Join(fakeRepo(t, false), "logs", "decisions.jsonl")
	for name, tt := range map[string]struct {
		path  string
		flags []string
	}{
		"new directory":               {filepath.Join(t.TempDir(), "logs", "decisions.jsonl"), []string{"--dry-run"}},
		"inside work tree with --yes": {inRepo, []string{"--dry-run", "--yes"}},
	} {
		r := runRecord(t, tt.path, append(tt.flags, recordDecisionFlags...)...)
		if out := decode[RecordOutput](t, r); r.code != 0 || !out.DryRun || !out.Recording || out.ID != "" {
			t.Errorf("%s: exit %d output %+v, want 0, a dry run and no id", name, r.code, out)
		}
		if _, err := os.Stat(filepath.Dir(tt.path)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s: a dry run created %s (stat err %v)", name, filepath.Dir(tt.path), err)
		}
	}
	if r := runRecord(t, inRepo, append([]string{"--dry-run"}, recordDecisionFlags...)...); r.code != 2 || !strings.Contains(r.stderr, "--yes") {
		t.Errorf("dry run inside a work tree without --yes: exit %d stderr %s, want 2 naming --yes, as the write would", r.code, r.stderr)
	}

	path, id := newLog(t)
	for _, args := range [][]string{
		recordDecisionFlags,
		{"record", "run", "--id", id, "--path", "fast", "--pre-screen", decidedPreScreen},
		{"record", "outcome", "--id", id, "--class", "contested", "--why", "Still split."},
	} {
		r := runRecord(t, path, append([]string{"--dry-run"}, args...)...)
		if out := decode[RecordOutput](t, r); r.code != 0 || !out.DryRun || out.ID != "" {
			t.Errorf("%v: exit %d output %+v, want 0, a dry run and no id", args[:2], r.code, out)
		}
	}
	if got := len(logLines(t, path)); got != 1 {
		t.Errorf("log has %d lines after dry runs, want 1", got)
	}

	r := runRecord(t, path, "--dry-run", "record", "run", "--id", "2026-01-01-0000", "--path", "fast", "--pre-screen", decidedPreScreen)
	failsNaming(t, r, "id", path, 1)
	r = runRecord(t, path, "--dry-run", "record", "outcome", "--id", id, "--class", "dominant", "--correct-option", "redis", "--why", "w")
	failsNaming(t, r, "correct-option", path, 1)
}

func TestRecordNeverWritesAnEnvironmentValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "decisions.jsonl")
	vars := map[string]string{
		decideLogEnv:            path,
		"TYPESAFE_API_KEY":      "jev-secret-key",
		"CLOUDFLARE_AUTH_TOKEN": "cf-secret-token",
		"CLOUDFLARE_ACCOUNT_ID": "cf-account-id",
	}
	for name, v := range vars {
		t.Setenv(name, v)
	}

	r := runRecordEnv(t, vars, recordDecisionFlags...)
	id := decode[RecordOutput](t, r).ID
	for _, args := range [][]string{
		{"record", "run", "--id", id, "--path", "fast", "--pre-screen", decidedPreScreen},
		{"record", "outcome", "--id", id, "--class", "dominant", "--correct-option", "postgres", "--why", "Held up."},
	} {
		if r := runRecordEnv(t, vars, args...); r.code != 0 {
			t.Fatalf("%v exit %d: %s", args[:2], r.code, r.stderr)
		}
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for name, v := range vars {
		if bytes.Contains(raw, []byte(v)) {
			t.Errorf("the log holds the value of %s", name)
		}
	}
}
