package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const clefAccount = "0123456789abcdef0123456789abcdef"

type backendRun struct {
	run
	paths []string
}

func clefEnvelope(answers string) string {
	return `{"result":` + answers + `,"success":true,"errors":[],"messages":[]}`
}

func executeBackend(t *testing.T, args []string, status int, body string, setup func(t *testing.T)) backendRun {
	t.Helper()
	var hits atomic.Int32
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		paths = append(paths, r.URL.Path)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("TYPESAFE_API_KEY", "test-key")
	t.Setenv("TYPESAFE_BASE_URL", srv.URL)
	t.Setenv("TYPESAFE_LOG_LEVEL", "")
	t.Setenv("CLOUDFLARE_AUTH_TOKEN", "cf-token")
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", clefAccount)
	t.Setenv("CLOUDFLARE_BASE_URL", srv.URL)
	t.Setenv("CLOUDFLARE_LOG_LEVEL", "")
	if setup != nil {
		setup(t)
	}

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), args, Env{
		Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr,
		Getenv: func(string) string { return "" }, Version: goldenVersion,
	})
	return backendRun{run: run{stdout: stdout.String(), stderr: stderr.String(), code: code, hits: hits.Load()}, paths: paths}
}

func with(args []string, extra ...string) []string {
	return append(append([]string{}, args...), extra...)
}

func TestBackendDefaultsToJev(t *testing.T) {
	r := executeBackend(t, askFlags, http.StatusOK, choiceAnswer("ship", 0.95), nil)
	out := decode[AskOutput](t, r.run)
	if r.code != 0 || out.Backend != "jev" || len(r.paths) != 1 || r.paths[0] != "/v1/systemone" {
		t.Fatalf("exit %d backend %q paths %v\nstderr: %s", r.code, out.Backend, r.paths, r.stderr)
	}
}

func TestClefBackendAsksCloudflareAndNamesItself(t *testing.T) {
	wantPath := "/client/v4/accounts/" + clefAccount + "/ai/run/@cf/cloudflare/clef"
	tests := []struct {
		name    string
		args    []string
		body    string
		backend func(t *testing.T, r run) string
	}{
		{"ask", with(askFlags, "--backend", "clef"), clefEnvelope(choiceAnswer("ship", 0.95)), func(t *testing.T, r run) string {
			return decode[AskOutput](t, r).Backend
		}},
		{"score", with(scoreFlags, "--backend", "clef"), clefEnvelope(scoreAnswer(2, 0.95)), func(t *testing.T, r run) string {
			return decode[ScoreOutput](t, r).Backend
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := executeBackend(t, tt.args, http.StatusOK, tt.body, nil)
			if r.code != 0 || tt.backend(t, r.run) != "clef" {
				t.Fatalf("exit %d\nstdout: %s\nstderr: %s", r.code, r.stdout, r.stderr)
			}
			if len(r.paths) != 1 || r.paths[0] != wantPath {
				t.Errorf("paths = %v, want [%s]", r.paths, wantPath)
			}
		})
	}
}

func TestUnknownBackendFailsBeforeAnyRequest(t *testing.T) {
	for _, cmd := range [][]string{askFlags, scoreFlags, {"eval", "--decisions", "x", "--truth", "y"}} {
		t.Run(cmd[0], func(t *testing.T) {
			r := executeBackend(t, with(cmd, "--backend", "gpt"), http.StatusOK, "", nil)
			if r.code != 2 || r.hits != 0 || r.stdout != "" {
				t.Fatalf("exit %d hits %d stdout %q, want exit 2, no request and no result", r.code, r.hits, r.stdout)
			}
			if !strings.Contains(r.stderr, "validation_error") || !strings.Contains(r.stderr, "jev, clef") {
				t.Errorf("stderr = %s, want a validation error listing the allowed values", r.stderr)
			}
		})
	}
}

func TestMissingClefCredentialsNeverFallBackToJev(t *testing.T) {
	r := executeBackend(t, with(askFlags, "--backend", "clef"), http.StatusOK, choiceAnswer("ship", 0.95), func(t *testing.T) {
		t.Setenv("CLOUDFLARE_AUTH_TOKEN", "")
	})
	if r.code == 0 || r.hits != 0 || r.stdout != "" {
		t.Fatalf("exit %d hits %d stdout %q, want a failure with no request to either backend", r.code, r.hits, r.stdout)
	}
	if !strings.Contains(r.stderr, "CLOUDFLARE_AUTH_TOKEN") || !strings.Contains(r.stderr, "clef.") {
		t.Errorf("stderr = %s, want a clef error naming the variable", r.stderr)
	}
}

func TestClefDryRunNeedsNoCredentials(t *testing.T) {
	r := executeBackend(t, with(askFlags, "--backend", "clef", "--dry-run"), http.StatusOK, "", func(t *testing.T) {
		t.Setenv("CLOUDFLARE_AUTH_TOKEN", "")
		t.Setenv("CLOUDFLARE_ACCOUNT_ID", "")
		t.Setenv("TYPESAFE_API_KEY", "")
	})
	out := decode[DryRunOutput](t, r.run)
	if r.code != 0 || r.hits != 0 || out.Backend != "clef" || !out.DryRun {
		t.Fatalf("exit %d hits %d out %+v\nstderr: %s", r.code, r.hits, out, r.stderr)
	}
}

func TestClefFailuresKeepClefCodesAndExitCodes(t *testing.T) {
	r := executeBackend(t, with(askFlags, "--backend", "clef"), http.StatusUnauthorized,
		`{"result":null,"success":false,"errors":[{"code":10000,"message":"Authentication error"}],"messages":[]}`, nil)
	if r.code != 4 || !strings.Contains(r.stderr, "clef.unauthorized") || strings.Contains(r.stderr, "jev.") {
		t.Fatalf("exit %d, want 4 with clef.unauthorized\nstderr: %s", r.code, r.stderr)
	}
}

func TestThresholdFlagsOverrideTheBackendDefault(t *testing.T) {
	r := executeBackend(t, with(askFlags, "--backend", "clef", "--dry-run", "--confident", "0.95"), http.StatusOK, "", nil)
	out := decode[DryRunOutput](t, r.run)
	if r.code != 0 || out.Thresholds != (ThresholdsOutput{Floor: 0.5, Confident: 0.95}) {
		t.Fatalf("exit %d thresholds %+v", r.code, out.Thresholds)
	}
}
