package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/kataras/jev"
	"github.com/rshade/ax-go/contract"

	"github.com/rshade/jev-decide/jevclient"
)

func evalAnswer(choice string, confidence float64) string {
	return fmt.Sprintf(`{"model":"jev-latest","answers":{"choice":{"type":"choice","choice":%q,`+
		`"confidence":%v,"probabilities":{"a":0.6,"b":0.4}}},`+
		`"usage":{"input_tokens":5,"output_tokens":1}}`, choice, confidence)
}

const evalNoAnswer = `{"model":"jev-latest","answers":{"other":{"type":"noul","noul":0.5}},"usage":{"input_tokens":5,"output_tokens":1}}`

// evalServer answers request i with replies[i], and repeats the last reply
// after that. A reply of "" is a 500.
func evalServer(t *testing.T, replies ...string) (*httptest.Server, *atomic.Int32, *[]string) {
	t.Helper()
	var hits atomic.Int32
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := int(hits.Add(1)) - 1
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		mu.Unlock()
		reply := replies[min(i, len(replies)-1)]
		if reply == "" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"detail":{"error_type":"x","message":"m"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits, &bodies
}

func evalArgs(t *testing.T, extra ...string) []string {
	t.Helper()
	return append([]string{"eval", "--decisions", writeFile(t, twoDecisions), "--truth", writeFile(t, twoTruths)}, extra...)
}

func TestEvalPrintsTheReport(t *testing.T) {
	srv, hits, _ := evalServer(t, evalAnswer("a", 0.95), evalAnswer("a", 0.6))
	r := executeAt(t, srv.URL, "", evalArgs(t))
	if r.code != 0 || hits.Load() != 2 {
		t.Fatalf("exit %d with %d requests, stderr %s; want exit 0 and 2 requests", r.code, hits.Load(), r.stderr)
	}

	out := decode[EvalOutput](t, r)
	if out.SchemaVersion != SchemaVersion || len(out.Decisions) != 2 {
		t.Fatalf("output = %+v, want the schema version and 2 rows", out)
	}
	if out.Metrics.Accuracy == nil || *out.Metrics.Accuracy != 1 || out.Metrics.ContestedAUC == nil || *out.Metrics.ContestedAUC != 1 {
		t.Errorf("metrics = %+v, want accuracy 1 and AUC 1", out.Metrics)
	}
	if want := (0.05*0.05 + 0.6*0.6) / 2; math.Abs(out.Metrics.Brier-want) > 1e-12 {
		t.Errorf("Brier = %v, want %v", out.Metrics.Brier, want)
	}
	if out.Outcomes != (EvalOutcomesOutput{Decided: 1, Uncertain: 1}) {
		t.Errorf("outcomes = %+v, want 1 decided and 1 uncertain", out.Outcomes)
	}
	d1, d2 := out.Decisions[0], out.Decisions[1]
	if d1.ID != "d1" || d1.Pick != "a" || !d1.Safe || d1.Outcome != "decided" || d1.CorrectOption == nil || *d1.CorrectOption != "a" {
		t.Errorf("d1 row = %+v", d1)
	}
	if d2.ID != "d2" || d2.Safe || d2.Outcome != "uncertain" || d2.CorrectOption != nil || d2.Class != "contested" {
		t.Errorf("d2 row = %+v", d2)
	}
	if len(out.ByThreshold) != 19 {
		t.Errorf("%d threshold rows, want 19: the default levels are on the grid", len(out.ByThreshold))
	}
	i := slices.IndexFunc(out.ByThreshold, func(r EvalThresholdRow) bool { return r.Threshold == 0.65 })
	if i < 0 || out.ByThreshold[i].Selected != 1 || out.ByThreshold[i].Unsafe != 0 {
		t.Errorf("0.65 row at %d = %+v, want only d1 selected", i, out.ByThreshold)
	}
}

// firstOptionServer answers every choice question with its first option, in
// name order, at the given confidence.
func firstOptionServer(t *testing.T, confidence float64) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		var req struct {
			Questions struct {
				Choice struct {
					Criteria map[string]string `json:"criteria"`
				} `json:"choice"`
			} `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Questions.Choice.Criteria) == 0 {
			t.Errorf("request has no choice criteria: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		names := slices.Sorted(maps.Keys(req.Questions.Choice.Criteria))
		probabilities := map[string]float64{}
		for _, n := range names {
			probabilities[n] = (1 - confidence) / float64(len(names)-1)
		}
		probabilities[names[0]] = confidence
		body, _ := json.Marshal(map[string]any{
			"model": "jev-latest",
			"answers": map[string]any{"choice": map[string]any{
				"type": "choice", "choice": names[0], "confidence": confidence, "probabilities": probabilities,
			}},
			"usage": map[string]int{"input_tokens": 5, "output_tokens": 1},
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestEvalReportForTheBundledSet(t *testing.T) {
	srv, hits := firstOptionServer(t, 0.95)
	r := executeAt(t, srv.URL, "", []string{"eval", "--decisions", bundledDecisions, "--truth", bundledTruth})
	if r.code != 0 || hits.Load() != 40 {
		t.Fatalf("exit %d with %d requests, stderr %s; want exit 0 and 40 requests", r.code, hits.Load(), r.stderr)
	}
	out := decode[EvalOutput](t, r)
	if len(out.Decisions) != 40 || out.Metrics.Accuracy == nil || out.Metrics.ContestedAUC == nil || len(out.ByThreshold) != 19 {
		t.Fatalf("report = %d rows, metrics %+v, %d threshold rows; want 40 rows, accuracy and AUC set, 19 rows",
			len(out.Decisions), out.Metrics, len(out.ByThreshold))
	}
}

func TestEvalExitsZeroWhateverTheMetrics(t *testing.T) {
	srv, _, _ := evalServer(t, evalAnswer("a", 0.6))
	r := executeAt(t, srv.URL, "", evalArgs(t))
	out := decode[EvalOutput](t, r)
	if r.code != 0 || out.Metrics.ContestedAUC == nil || *out.Metrics.ContestedAUC != 0.5 || out.Outcomes.Uncertain != 2 {
		t.Fatalf("exit %d, metrics %+v, outcomes %+v; want exit 0 with AUC 0.5 and both uncertain", r.code, out.Metrics, out.Outcomes)
	}
}

func TestEvalFailsWholeOnAMissingAnswer(t *testing.T) {
	srv, hits, _ := evalServer(t, evalAnswer("a", 0.95), evalNoAnswer)
	r := executeAt(t, srv.URL, "", evalArgs(t))
	if r.code == 0 || r.stdout != "" || hits.Load() != 2 {
		t.Fatalf("exit %d, %d requests, stdout %q; want a failure, 2 requests and no report", r.code, hits.Load(), r.stdout)
	}
	if !strings.Contains(r.stderr, "decision d2") || !strings.Contains(r.stderr, `"decision":"d2"`) {
		t.Fatalf("stderr %s does not name d2", r.stderr)
	}
}

func TestEvalCacheMakesARerunFree(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "cache")
	srv, hits, _ := evalServer(t, evalAnswer("a", 0.95), evalAnswer("a", 0.6))
	args := evalArgs(t, "--cache-dir", cache)

	first := executeAt(t, srv.URL, "", args)
	second := executeAt(t, srv.URL, "", args)
	if first.code != 0 || second.code != 0 || hits.Load() != 2 {
		t.Fatalf("exits %d and %d with %d requests; want 0, 0 and 2", first.code, second.code, hits.Load())
	}
	if !reflect.DeepEqual(decode[EvalOutput](t, first).Metrics, decode[EvalOutput](t, second).Metrics) {
		t.Fatal("the cached rerun reported different metrics")
	}
}

func TestEvalRerunAfterAFailureSkipsWhatWasAnswered(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "cache")
	srv, hits, _ := evalServer(t, evalAnswer("a", 0.95), "", evalAnswer("a", 0.6))
	args := evalArgs(t, "--cache-dir", cache)

	if r := executeAt(t, srv.URL, "", args); r.code == 0 {
		t.Fatal("first run succeeded, want the 500 to fail it")
	}
	if r := executeAt(t, srv.URL, "", args); r.code != 0 {
		t.Fatalf("second run exit %d, stderr %s", r.code, r.stderr)
	}
	if got := hits.Load(); got != 3 {
		t.Fatalf("server received %d requests, want 3: d1 is not asked again", got)
	}
}

func TestEvalRerunRecoversFromAMissingAnswer(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "cache")
	srv, hits, _ := evalServer(t, evalAnswer("a", 0.95), evalNoAnswer, evalAnswer("a", 0.6))
	args := evalArgs(t, "--cache-dir", cache)

	if r := executeAt(t, srv.URL, "", args); r.code == 0 {
		t.Fatal("first run succeeded, want the missing answer to fail it")
	}
	if r := executeAt(t, srv.URL, "", args); r.code != 0 {
		t.Fatalf("second run exit %d, stderr %s: the bad response was replayed from the cache", r.code, r.stderr)
	}
	if got := hits.Load(); got != 3 {
		t.Fatalf("server received %d requests, want 3: d2 is asked again, d1 is not", got)
	}
}

func TestEvalWithoutCacheWritesNothing(t *testing.T) {
	args := evalArgs(t)
	wd := t.TempDir()
	t.Chdir(wd)
	srv, _, _ := evalServer(t, evalAnswer("a", 0.95))
	if r := executeAt(t, srv.URL, "", args); r.code != 0 {
		t.Fatalf("exit %d, stderr %s", r.code, r.stderr)
	}
	if entries, _ := os.ReadDir(wd); len(entries) != 0 {
		t.Fatalf("eval wrote %d files without --cache-dir", len(entries))
	}
}

func TestEvalSendsTheInstructionsEveryTime(t *testing.T) {
	srv, _, bodies := evalServer(t, evalAnswer("a", 0.95))
	if r := executeAt(t, srv.URL, "", evalArgs(t, "--instructions", "Which option fits best?")); r.code != 0 {
		t.Fatalf("exit %d, stderr %s", r.code, r.stderr)
	}
	if len(*bodies) != 2 {
		t.Fatalf("%d requests, want 2", len(*bodies))
	}
	for i, b := range *bodies {
		if !strings.Contains(b, "Which option fits best?") {
			t.Fatalf("request %d = %s, want the custom instructions", i, b)
		}
	}
	if !strings.Contains((*bodies)[0], `"pros":["cheap"]`) {
		t.Fatalf("request for d1 = %s, want its option pros in the state", (*bodies)[0])
	}
}

func TestEvalInvalidInputSendsNothing(t *testing.T) {
	srv, hits, _ := evalServer(t, evalAnswer("a", 0.95))
	truth := writeFile(t, strings.Replace(twoTruths, `"correct_option":"a"`, `"correct_option":"Mainframe"`, 1))
	r := executeAt(t, srv.URL, "", []string{"eval", "--decisions", writeFile(t, twoDecisions), "--truth", truth})
	if r.code != contract.ExitValidation || hits.Load() != 0 || r.stdout != "" {
		t.Fatalf("exit %d, %d requests, stdout %q; want exit 2, none and no report", r.code, hits.Load(), r.stdout)
	}
	if !strings.Contains(r.stderr, `"field":"truth[d1].correct_option"`) {
		t.Fatalf("stderr %s does not name truth[d1].correct_option", r.stderr)
	}
}

func TestEvalDryRunNeedsNoKeyAndSendsNothing(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), append([]string{"--dry-run"}, evalArgs(t)...), Env{
		Stdout: &stdout, Stderr: &stderr, Getenv: func(string) string { return "" }, Version: goldenVersion,
		NewClient: func(...jevclient.Option) (*jev.Client, error) {
			t.Error("a dry run built a client")
			return nil, errors.New("no client in a dry run")
		},
	})
	if code != 0 {
		t.Fatalf("exit %d, stderr %s", code, stderr.String())
	}
	out := decode[EvalDryRunOutput](t, run{stdout: stdout.String()})
	want := EvalDryRunOutput{SchemaVersion: SchemaVersion, DryRun: true, Decisions: 2, Dominant: 1, Contested: 1, Thresholds: ThresholdsOutput{Floor: 0.5, Confident: 0.9}}
	if out != want {
		t.Fatalf("dry run = %+v, want %+v", out, want)
	}
}

func TestEvalDryRunOfTheBundledSet(t *testing.T) {
	r := executeAt(t, "http://127.0.0.1:1", "", []string{"--dry-run", "eval", "--decisions", bundledDecisions, "--truth", bundledTruth})
	out := decode[EvalDryRunOutput](t, r)
	if r.code != 0 || out.Decisions != 40 || out.Dominant != 20 || out.Contested != 20 {
		t.Fatalf("exit %d, dry run %+v; want 40 decisions, 20 and 20", r.code, out)
	}
}
