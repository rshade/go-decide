package cli

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ax "github.com/rshade/ax-go"
)

var fixedNow = time.Date(2026, 10, 12, 14, 3, 0, 0, time.UTC)

// testRecorder returns a recorder whose DECIDE_LOG is path ("" for unset),
// with a fixed clock and a seeded id source.
func testRecorder(path string) *recorder {
	return newRecorder(Env{
		Getenv: func(key string) string {
			if key == decideLogEnv {
				return path
			}
			return ""
		},
		Now:  func() time.Time { return fixedNow },
		Rand: rand.NewChaCha8([32]byte{1}),
	})
}

func appendDecisionLine(t *testing.T, r *recorder, ctx context.Context) logLine {
	t.Helper()
	line, err := r.commit(ctx, func(lines []logLine) (logLine, error) {
		id, err := r.newDecisionID(lines)
		if err != nil {
			return logLine{}, err
		}
		return logLine{Kind: kindDecision, ID: id, Decision: json.RawMessage(`{"id":"` + id + `"}`)}, nil
	})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	return line
}

func readLog(t *testing.T, path string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("log line %q is not JSON: %v", l, err)
		}
		out = append(out, m)
	}
	return out
}

func TestRecorderIsOffWithoutDecideLog(t *testing.T) {
	if testRecorder("").on() {
		t.Error("recorder is on with DECIDE_LOG unset, want off")
	}
}

func TestRecorderCreatesAnOwnerOnlyLog(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state", "decide")
	path := filepath.Join(dir, "decisions.jsonl")
	appendDecisionLine(t, testRecorder(path), context.Background())

	for p, want := range map[string]os.FileMode{path: 0o600, dir: 0o700} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s mode = %o, want %o", p, got, want)
		}
	}
}

func TestRecorderStampsEveryLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "decisions.jsonl")
	line := appendDecisionLine(t, testRecorder(path), context.Background())

	got := readLog(t, path)
	if len(got) != 1 {
		t.Fatalf("log has %d lines, want 1", len(got))
	}
	for _, key := range []string{"v", "kind", "id", "recorded_at", "trace_id"} {
		if _, ok := got[0][key]; !ok {
			t.Errorf("line has no %q: %v", key, got[0])
		}
	}
	if got[0]["v"] != float64(1) || got[0]["recorded_at"] != "2026-10-12T14:03:00Z" || got[0]["id"] != line.ID {
		t.Errorf("line = %v, want v 1, recorded_at 2026-10-12T14:03:00Z and id %s", got[0], line.ID)
	}
	if !strings.HasPrefix(line.ID, "2026-10-12-") || len(line.ID) != len("2026-10-12-a1b2") {
		t.Errorf("id %q, want the UTC date and four hex characters", line.ID)
	}
}

func TestRecorderKeepsEarlierLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "decisions.jsonl")
	earlier := `{"v":1,"kind":"decision","id":"2026-01-01-aaaa","recorded_at":"2026-01-01T00:00:00Z","trace_id":"x","decision":{"id":"2026-01-01-aaaa"}}` + "\n"
	if err := os.WriteFile(path, []byte(earlier), 0o600); err != nil {
		t.Fatal(err)
	}
	appendDecisionLine(t, testRecorder(path), context.Background())

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), earlier) || strings.Count(string(raw), "\n") != 2 {
		t.Errorf("log = %q, want the earlier line unchanged and one new line", raw)
	}
}

func TestRecorderNeverReusesADecisionID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "decisions.jsonl")
	seen := map[string]bool{}
	for range 50 {
		r := testRecorder(path) // same seed, so every draw starts with the same id
		line := appendDecisionLine(t, r, context.Background())
		if seen[line.ID] {
			t.Fatalf("id %s was issued twice", line.ID)
		}
		seen[line.ID] = true
	}
}

func TestRecorderReturnsAWriteFailure(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	r := testRecorder(filepath.Join(blocker, "decisions.jsonl"))
	_, err := r.commit(context.Background(), func([]logLine) (logLine, error) {
		return logLine{Kind: kindDecision, ID: "x"}, nil
	})
	if err == nil {
		t.Fatal("commit under a regular file succeeded, want an error")
	}
}

func TestRecorderReadsAMissingLogAsEmpty(t *testing.T) {
	lines, err := readLogLines(filepath.Join(t.TempDir(), "absent.jsonl"))
	if err != nil || len(lines) != 0 {
		t.Errorf("readLogLines(absent) = %v, %v; want no lines and no error", lines, err)
	}
}

func TestRecorderNamesAMalformedLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "decisions.jsonl")
	content := `{"v":1,"kind":"decision","id":"a","recorded_at":"2026-01-01T00:00:00Z"}` + "\nnot json\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := readLogLines(path)
	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Errorf("readLogLines error = %v, want one naming line 2", err)
	}
}

// fakeRepo returns a directory that git would treat as a work tree root.
func fakeRepo(t *testing.T, asFile bool) string {
	t.Helper()
	repo := t.TempDir()
	marker := filepath.Join(repo, ".git")
	var err error
	if asFile {
		err = os.WriteFile(marker, []byte("gitdir: elsewhere\n"), 0o600)
	} else {
		err = os.Mkdir(marker, 0o700)
	}
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestRecorderRefusesAWorkTreeWithoutYes(t *testing.T) {
	for name, asFile := range map[string]bool{".git directory": false, ".git file": true} {
		t.Run(name, func(t *testing.T) {
			repo := fakeRepo(t, asFile)
			missing := filepath.Join(repo, "not-yet")
			path := filepath.Join(missing, "logs", "decisions.jsonl")
			_, err := testRecorder(path).commit(context.Background(), func([]logLine) (logLine, error) {
				return logLine{Kind: kindDecision, ID: "x"}, nil
			})
			var axErr *ax.Error
			if !errors.As(err, &axErr) || ax.ErrorExitCode(err) != ax.ExitValidation || !strings.Contains(axErr.Error()+axErr.ActionableFix, "--yes") {
				t.Fatalf("commit error = %v, want a validation error naming --yes", err)
			}
			if _, err := os.Stat(missing); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("refused commit created %s (stat err %v)", missing, err)
			}
		})
	}
}

func TestRecorderWritesInAWorkTreeWithYes(t *testing.T) {
	path := filepath.Join(fakeRepo(t, false), "logs", "decisions.jsonl")
	appendDecisionLine(t, testRecorder(path), ax.WithApproval(context.Background(), true))
	if _, err := os.Stat(path); err != nil {
		t.Errorf("approved commit did not write %s: %v", path, err)
	}
}
