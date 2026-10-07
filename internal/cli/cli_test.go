package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kataras/jev"
	"github.com/rshade/ax-go/contract"

	"github.com/rshade/go-decide/jevclient"
)

const goldenVersion = "v9.9.9-golden"

type run struct {
	stdout, stderr string
	code           int
	hits           int32
}

type envelope[T any] struct {
	Data T `json:"data"`
}

func choiceAnswer(choice string, confidence float64) string {
	return fmt.Sprintf(`{"model":"jev-latest","answers":{"choice":{"type":"choice","choice":%q,`+
		`"confidence":%v,"probabilities":{"ship":0.6,"hold":0.4}}},`+
		`"usage":{"input_tokens":5,"output_tokens":1}}`, choice, confidence)
}

func scoreAnswer(score, confidence float64) string {
	return fmt.Sprintf(`{"model":"jev-latest","answers":{"score":{"type":"score","score":%v,"confidence":%v,`+
		`"legend":{"0":"a","1":"b","2":"c"},"probabilities":{"0":0.05,"1":0.15,"2":0.8}}},`+
		`"usage":{"input_tokens":5,"output_tokens":1}}`, score, confidence)
}

func execute(t *testing.T, stdin string, args []string, status int, body string) run {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	r := executeAt(t, srv.URL, stdin, args)
	r.hits = hits.Load()
	return r
}

func executeAt(t *testing.T, baseURL, stdin string, args []string) run {
	t.Helper()
	t.Setenv("TYPESAFE_API_KEY", "test-key")
	t.Setenv("TYPESAFE_BASE_URL", baseURL)
	t.Setenv("TYPESAFE_DEFAULT_MODEL", "")
	t.Setenv("TYPESAFE_LOG_LEVEL", "")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), args, Env{
		Stdin:   strings.NewReader(stdin),
		Stdout:  &stdout,
		Stderr:  &stderr,
		Getenv:  func(string) string { return "" },
		Version: goldenVersion,
	})
	return run{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func decode[T any](t *testing.T, r run) T {
	t.Helper()
	var env envelope[T]
	if err := json.Unmarshal([]byte(r.stdout), &env); err != nil {
		t.Fatalf("decoding stdout %q: %v", r.stdout, err)
	}
	return env.Data
}

var (
	askFlags   = []string{"ask", "--state", "all checks passed", "--instructions", "Ship it?", "--option", "ship=release it", "--option", "hold=wait"}
	scoreFlags = []string{"score", "--state", "checkout fails", "--instructions", "How bad?", "--level", "a=minor", "--level", "b=noticeable", "--level", "c=severe"}
)

func TestAskOutcomesAndExitCodes(t *testing.T) {
	tests := []struct {
		name       string
		confidence float64
		args       []string
		wantOut    string
		wantCode   int
	}{
		{"decided", 0.95, nil, "decided", 0},
		{"uncertain", 0.7, nil, "uncertain", ExitUncertain},
		{"escalate", 0.3, nil, "escalate", ExitEscalate},
		{"custom thresholds decide", 0.85, []string{"--floor", "0.4", "--confident", "0.8"}, "decided", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := execute(t, "", append(append([]string{}, askFlags...), tt.args...), http.StatusOK, choiceAnswer("ship", tt.confidence))

			out := decode[AskOutput](t, r)
			if r.code != tt.wantCode || out.Outcome != tt.wantOut {
				t.Fatalf("exit %d outcome %q, want %d and %q\nstdout: %s\nstderr: %s", r.code, out.Outcome, tt.wantCode, tt.wantOut, r.stdout, r.stderr)
			}
			if r.stderr != "" {
				t.Errorf("stderr = %q, want nothing for an outcome that is not an error", r.stderr)
			}
			if out.SchemaVersion != SchemaVersion {
				t.Errorf("schema_version = %d, want %d", out.SchemaVersion, SchemaVersion)
			}
			if (out.Choice != "") != (tt.wantOut == "decided") || (out.Leading != "") == (tt.wantOut == "decided") {
				t.Errorf("choice %q leading %q for a %s outcome: only a decided outcome may name a choice", out.Choice, out.Leading, tt.wantOut)
			}
		})
	}
}

func TestHumanModePrintsTheSameEnvelope(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		body string
	}{
		{"ask", askFlags, choiceAnswer("ship", 0.95)},
		{"score", scoreFlags, scoreAnswer(1.9, 0.95)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			machine := execute(t, "", append(append([]string{}, tt.args...), "--format", "json"), http.StatusOK, tt.body)
			human := execute(t, "", append(append([]string{}, tt.args...), "--format", "human"), http.StatusOK, tt.body)

			if human.code != 0 || machine.code != 0 {
				t.Fatalf("exit codes %d (human) and %d (json), want 0\nstderr: %s", human.code, machine.code, human.stderr)
			}
			var humanEnv, machineEnv envelope[json.RawMessage]
			if err := json.Unmarshal([]byte(human.stdout), &humanEnv); err != nil {
				t.Fatalf("human mode stdout is not a JSON envelope: %v\n%s", err, human.stdout)
			}
			if err := json.Unmarshal([]byte(machine.stdout), &machineEnv); err != nil {
				t.Fatalf("json mode stdout is not a JSON envelope: %v\n%s", err, machine.stdout)
			}
			if string(humanEnv.Data) != string(machineEnv.Data) {
				t.Errorf("human mode data differs from json mode:\nhuman: %s\njson:  %s", humanEnv.Data, machineEnv.Data)
			}
		})
	}
}

func TestAskEchoesTheThresholdsUsed(t *testing.T) {
	r := execute(t, "", append(append([]string{}, askFlags...), "--floor", "0.4", "--confident", "0.8"), http.StatusOK, choiceAnswer("ship", 0.85))

	if got := decode[AskOutput](t, r).Thresholds; got != (ThresholdsOutput{Floor: 0.4, Confident: 0.8}) {
		t.Fatalf("thresholds = %+v, want 0.4 and 0.8", got)
	}
	r = execute(t, "", askFlags, http.StatusOK, choiceAnswer("ship", 0.95))
	if got := decode[AskOutput](t, r).Thresholds; got != (ThresholdsOutput{Floor: 0.5, Confident: 0.9}) {
		t.Fatalf("default thresholds = %+v, want 0.5 and 0.9", got)
	}
}

func TestScoreAcceptsCustomThresholds(t *testing.T) {
	r := execute(t, "", append(append([]string{}, scoreFlags...), "--floor", "0.4", "--confident", "0.8"), http.StatusOK, scoreAnswer(1.9, 0.85))

	out := decode[ScoreOutput](t, r)
	if r.code != 0 || out.Outcome != "decided" || out.Thresholds != (ThresholdsOutput{Floor: 0.4, Confident: 0.8}) {
		t.Fatalf("exit %d outcome %q thresholds %+v, want a decided outcome with 0.4 and 0.8\nstdout: %s", r.code, out.Outcome, out.Thresholds, r.stdout)
	}
}

func TestScoreOutcomesAndExitCodes(t *testing.T) {
	tests := []struct {
		name       string
		score      float64
		confidence float64
		wantOut    string
		wantCode   int
		wantLevel  string
	}{
		{"decided", 1.9, 0.95, "decided", 0, "c"},
		{"uncertain", 1.9, 0.7, "uncertain", ExitUncertain, "c"},
		{"escalate", 1.9, 0.3, "escalate", ExitEscalate, "c"},
		{"in between rounds down", 0.5, 0.95, "decided", 0, "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := execute(t, "", scoreFlags, http.StatusOK, scoreAnswer(tt.score, tt.confidence))

			out := decode[ScoreOutput](t, r)
			if r.code != tt.wantCode || out.Outcome != tt.wantOut || out.Score != tt.score {
				t.Fatalf("exit %d outcome %q score %v, want %d %q %v\nstdout: %s\nstderr: %s", r.code, out.Outcome, out.Score, tt.wantCode, tt.wantOut, tt.score, r.stdout, r.stderr)
			}
			level, nearest := out.Level, out.Nearest
			if tt.wantOut != "decided" {
				level, nearest = out.Nearest, out.Level
			}
			if level != tt.wantLevel || nearest != "" {
				t.Errorf("level %q and other %q, want %q and nothing in the other field: only a decided outcome may name a level", level, nearest, tt.wantLevel)
			}
			if r.stderr != "" {
				t.Errorf("stderr = %q, want nothing", r.stderr)
			}
		})
	}
}

func TestSpecFromFileStdinAndFlags(t *testing.T) {
	doc := `{"state":"all checks passed","instructions":"Ship it?","options":[{"name":"ship"},{"name":"hold"}]}`
	path := filepath.Join(t.TempDir(), "spec.json")
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatalf("writing the spec: %v", err)
	}
	tests := []struct {
		name  string
		stdin string
		args  []string
	}{
		{"file", "", []string{"ask", "--spec", path}},
		{"stdin", doc, []string{"ask", "--spec", "-"}},
		{"file with a flag filling the state", "", []string{"ask", "--spec", writeSpec(t, `{"options":[{"name":"ship"},{"name":"hold"}]}`), "--state", "ok"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := execute(t, tt.stdin, tt.args, http.StatusOK, choiceAnswer("ship", 0.95))

			if r.code != 0 || decode[AskOutput](t, r).Choice != "ship" {
				t.Fatalf("exit %d, stdout %s, stderr %s; want a decided ship", r.code, r.stdout, r.stderr)
			}
		})
	}
}

func writeSpec(t *testing.T, doc string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "spec.json")
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatalf("writing the spec: %v", err)
	}
	return path
}

func TestInvalidInputFailsWithExitTwoAndNoRequest(t *testing.T) {
	two := `{"state":"s","options":[{"name":"a"},{"name":"b"}]}`
	levels := `{"state":"s","levels":[{"name":"a"},{"name":"b"}]}`
	tests := []struct {
		name      string
		stdin     string
		args      []string
		wantField string
	}{
		{"duplicate option names", `{"state":"s","options":[{"name":"a"},{"name":"b"},{"name":"a"}]}`, []string{"ask", "--spec", "-"}, "Options"},
		{"duplicate level names", `{"state":"s","levels":[{"name":"a"},{"name":"b"},{"name":"a"}]}`, []string{"score", "--spec", "-"}, "Levels"},
		{"duplicate names from flags", "", []string{"ask", "--state", "s", "--option", "a=1", "--option", "a=2"}, "Options"},
		{"no state", "", []string{"ask", "--option", "a=1", "--option", "b=2"}, "State"},
		{"one option", "", []string{"ask", "--state", "s", "--option", "a=1"}, "Options"},
		{"unknown field", `{"state":"s","color":"red"}`, []string{"ask", "--spec", "-"}, "color"},
		{"malformed document", `{"state":`, []string{"ask", "--spec", "-"}, "Spec"},
		{"missing spec file", "", []string{"ask", "--spec", "/nonexistent/spec.json"}, ""},
		{"levels given to ask", levels, []string{"ask", "--spec", "-"}, "Options"},
		{"options given to score", two, []string{"score", "--spec", "-"}, "Levels"},
		{"options and levels given to ask", `{"state":"s","options":[{"name":"a"},{"name":"b"}],"levels":[{"name":"x"},{"name":"y"}]}`, []string{"ask", "--spec", "-"}, "Levels"},
		{"options and levels given to score", `{"state":"s","options":[{"name":"a"},{"name":"b"}],"levels":[{"name":"x"},{"name":"y"}]}`, []string{"score", "--spec", "-"}, "Options"},
		{"state in both", two, []string{"ask", "--spec", "-", "--state", "again"}, "State"},
		{"options in both", two, []string{"ask", "--spec", "-", "--option", "c=1"}, "Options"},
		{"option without a name", "", []string{"ask", "--state", "s", "--option", "=x"}, ""},
		{"floor above confident", "", append(append([]string{}, askFlags...), "--floor", "0.9", "--confident", "0.5"), ""},
		{"threshold out of range", "", append(append([]string{}, askFlags...), "--floor", "1.5"), ""},
		{"score: floor above confident", "", append(append([]string{}, scoreFlags...), "--floor", "0.9", "--confident", "0.5"), ""},
		{"score: threshold out of range", "", append(append([]string{}, scoreFlags...), "--confident", "1.5"), ""},
		{"too many levels", "", []string{"score", "--state", "s", "--level", "1=a", "--level", "2=a", "--level", "3=a", "--level", "4=a", "--level", "5=a", "--level", "6=a", "--level", "7=a", "--level", "8=a", "--level", "9=a", "--level", "10=a", "--level", "11=a"}, "Levels"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := execute(t, tt.stdin, tt.args, http.StatusOK, choiceAnswer("a", 0.95))

			if r.code != contract.ExitValidation || r.stdout != "" || r.hits != 0 {
				t.Fatalf("exit %d, stdout %q, %d requests; want exit 2, no stdout and no request\nstderr: %s", r.code, r.stdout, r.hits, r.stderr)
			}
			if tt.wantField != "" && !strings.Contains(r.stderr, `"field":"`+tt.wantField+`"`) {
				t.Errorf("error envelope does not name the field %q:\n%s", tt.wantField, r.stderr)
			}
		})
	}
}

func TestFailuresKeepTheSharedExitCodes(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantCode int
		wantErr  string
	}{
		{"unauthorized", http.StatusUnauthorized, `{"detail":{"error_type":"auth","message":"bad key"}}`, contract.ExitAuth, jevclient.CodeUnauthorized},
		{"invalid request", http.StatusUnprocessableEntity, `{"detail":[{"loc":["body"],"msg":"bad","type":"x"}]}`, contract.ExitValidation, jevclient.CodeInvalidRequest},
		{"invalid response", http.StatusOK, choiceAnswer("abandon", 0.95), contract.ExitInternal, jevclient.CodeInvalidResponse},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := execute(t, "", askFlags, tt.status, tt.body)

			if r.code != tt.wantCode || r.stdout != "" || !strings.Contains(r.stderr, tt.wantErr) {
				t.Fatalf("exit %d, stdout %q, stderr %s; want exit %d, empty stdout and %s", r.code, r.stdout, r.stderr, tt.wantCode, tt.wantErr)
			}
		})
	}
}

func TestAConnectionFailureExitsThree(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()

	r := executeAt(t, url, "", askFlags)

	if r.code != contract.ExitNetwork || r.stdout != "" || !strings.Contains(r.stderr, jevclient.CodeConnection) {
		t.Fatalf("exit %d, stdout %q, stderr %s; want exit 3, empty stdout and %s", r.code, r.stdout, r.stderr, jevclient.CodeConnection)
	}
}

func TestDryRunValidatesWithoutAsking(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantKind  string
		wantNames []string
	}{
		{"ask", append([]string{"--dry-run"}, askFlags...), "choice", []string{"ship", "hold"}},
		{"score", append([]string{"--dry-run"}, scoreFlags...), "score", []string{"a", "b", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := execute(t, "", tt.args, http.StatusOK, choiceAnswer("ship", 0.95))

			out := decode[DryRunOutput](t, r)
			if r.code != 0 || r.hits != 0 || !out.DryRun || out.Kind != tt.wantKind || !slices.Equal(out.Names, tt.wantNames) {
				t.Fatalf("exit %d, %d requests, output %+v; want exit 0, no request and the %s names %v in order", r.code, r.hits, out, tt.wantKind, tt.wantNames)
			}
			if out.SchemaVersion != SchemaVersion || out.Thresholds != (ThresholdsOutput{Floor: 0.5, Confident: 0.9}) {
				t.Errorf("dry run output = %+v, want the schema version and default thresholds", out)
			}
		})
	}
}

func TestDryRunNeedsNoKeyAndStillValidates(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "")
	env := func(stdout, stderr *bytes.Buffer) Env {
		return Env{
			Stdout: stdout, Stderr: stderr, Getenv: func(string) string { return "" }, Version: goldenVersion,
			NewClient: func(...jevclient.Option) (*jev.Client, error) {
				t.Error("a dry run built a client")
				return nil, errors.New("no client in a dry run")
			},
		}
	}

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), append([]string{"--dry-run"}, scoreFlags...), env(&stdout, &stderr))
	if code != 0 || !strings.Contains(stdout.String(), `"names":["a","b","c"]`) {
		t.Fatalf("exit %d, stdout %s, stderr %s; want a successful dry run without a key", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"--dry-run", "ask", "--state", "s", "--option", "a=1", "--option", "a=2"}, env(&stdout, &stderr))
	if code != contract.ExitValidation || stdout.Len() != 0 || !strings.Contains(stderr.String(), `"field":"Options"`) {
		t.Fatalf("exit %d, stdout %q, stderr %s; want exit 2 naming Options", code, stdout.String(), stderr.String())
	}
}

func TestSchemaListsBothCommands(t *testing.T) {
	r := execute(t, "", []string{"__schema"}, http.StatusOK, "")

	var schema struct {
		Tool    string `json:"tool"`
		Version string `json:"version"`
		Command struct {
			Commands []struct {
				Use   string `json:"use"`
				Long  string `json:"long"`
				Flags []struct {
					Name string `json:"name"`
				} `json:"flags"`
			} `json:"commands"`
		} `json:"command"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &schema); err != nil {
		t.Fatalf("decoding __schema %q: %v", r.stdout, err)
	}
	found := map[string]bool{}
	for _, c := range schema.Command.Commands {
		found[c.Use] = strings.Contains(c.Long, fmt.Sprintf("schema_version: %d", SchemaVersion))
	}
	if r.code != 0 || schema.Tool != "go-decide" || schema.Version != goldenVersion || !found["ask"] || !found["score"] {
		t.Fatalf("__schema = exit %d tool %q version %q commands %v; want both commands with the output version", r.code, schema.Tool, schema.Version, found)
	}
	if r.hits != 0 {
		t.Errorf("__schema made %d requests, want 0", r.hits)
	}
	wantFlags := map[string][]string{
		"ask":   {"spec", "state", "instructions", "option", "floor", "confident"},
		"score": {"spec", "state", "instructions", "level", "floor", "confident"},
	}
	for _, c := range schema.Command.Commands {
		have := map[string]bool{}
		for _, f := range c.Flags {
			have[f.Name] = true
		}
		for _, name := range wantFlags[c.Use] {
			if !have[name] {
				t.Errorf("__schema lists no --%s flag for %s", name, c.Use)
			}
		}
	}
}

func TestVersionFlagPrintsTheInjectedVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"--version"}, Env{
		Stdout: &stdout, Stderr: &stderr, Version: goldenVersion,
	})
	if code != 0 || stdout.String() != goldenVersion+"\n" || stderr.Len() != 0 {
		t.Fatalf("--version exit %d stdout %q stderr %q, want %s", code, stdout.String(), stderr.String(), goldenVersion)
	}
}
