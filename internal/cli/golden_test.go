package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

//nolint:gochecknoglobals // the golden update flag must be package-scoped for the flag package
var updateGolden = flag.Bool("update", false, "rewrite the golden files for the current schema version")

var maskedFields = regexp.MustCompile(`"(trace_id|span_id|idempotency_key)":"[^"]*"`)

func mask(b string) string {
	return maskedFields.ReplaceAllString(b, `"$1":"<masked>"`)
}

// goldenCases pins each output. since is the schema version an output first
// appeared in; it has a golden file for every version from there on.
var goldenCases = []struct {
	name   string
	since  int
	args   []string
	stdin  string
	body   string
	status int
}{
	{"ask.decided", 1, askFlags, "", choiceAnswer("ship", 0.95), http.StatusOK},
	{"ask.uncertain", 1, askFlags, "", choiceAnswer("hold", 0.7), http.StatusOK},
	{"ask.escalate", 1, askFlags, "", choiceAnswer("hold", 0.3), http.StatusOK},
	{"score.decided", 1, scoreFlags, "", scoreAnswer(1.9, 0.95), http.StatusOK},
	{"score.uncertain", 1, scoreFlags, "", scoreAnswer(1.2, 0.7), http.StatusOK},
	{"score.escalate", 1, scoreFlags, "", scoreAnswer(0.4, 0.3), http.StatusOK},
	{"ask.dryrun", 1, append([]string{"--dry-run"}, askFlags...), "", "", http.StatusOK},
	{"score.dryrun", 1, append([]string{"--dry-run"}, scoreFlags...), "", "", http.StatusOK},
	{"schema", 1, []string{"__schema"}, "", "", http.StatusOK},
	{"eval.report", 2, evalGoldenFlags, "", evalAnswer("a", 0.95), http.StatusOK},
	{"eval.dryrun", 2, append([]string{"--dry-run"}, evalGoldenFlags...), "", "", http.StatusOK},
}

var evalGoldenFlags = []string{"eval", "--decisions", "testdata/eval/decisions.json", "--truth", "testdata/eval/truth.json"}

func goldenPath(name string, version int) string {
	return filepath.Join("testdata", "golden", fmt.Sprintf("%s.v%d.json", name, version))
}

func TestOutputMatchesTheGoldenFileForTheCurrentVersion(t *testing.T) {
	for _, tc := range goldenCases {
		if tc.since > SchemaVersion {
			t.Fatalf("%s starts at version %d, after SchemaVersion %d", tc.name, tc.since, SchemaVersion)
		}
		t.Run(tc.name, func(t *testing.T) {
			r := execute(t, tc.stdin, tc.args, tc.status, tc.body)
			got := mask(r.stdout)
			path := goldenPath(tc.name, SchemaVersion)

			if *updateGolden {
				if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
					t.Fatalf("writing %s: %v", path, err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("no golden file for schema version %d at %s: %v\n"+
					"add one for the new version with: go test ./internal/cli -run Golden -update", SchemaVersion, path, err)
			}
			if got != string(want) {
				t.Fatalf("output no longer matches %s.\n"+
					"A change to an output shape needs a new SchemaVersion (now %d) and a new golden file; keep the old one.\n"+
					"want: %s\ngot:  %s", path, SchemaVersion, want, got)
			}
		})
	}
}

func TestGoldenFilesOfEarlierVersionsAreFrozen(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "golden", "*.v*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no golden files found: %v", err)
	}
	for _, file := range files {
		base := filepath.Base(file)
		i := strings.LastIndex(base, ".v")
		version, err := strconv.Atoi(strings.TrimSuffix(base[i+2:], ".json"))
		if err != nil {
			t.Fatalf("%s: cannot read its version: %v", base, err)
		}
		if version > SchemaVersion {
			t.Errorf("%s is for version %d, which is newer than SchemaVersion %d", base, version, SchemaVersion)
		}
		if strings.HasPrefix(base, "schema.") {
			continue
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("reading %s: %v", base, err)
		}
		var env envelope[struct {
			SchemaVersion int `json:"schema_version"`
		}]
		if err := json.Unmarshal(raw, &env); err != nil || env.Data.SchemaVersion != version {
			t.Errorf("%s carries schema_version %d, want %d: a golden file must keep the version in its name", base, env.Data.SchemaVersion, version)
		}
	}
	for _, tc := range goldenCases {
		for v := tc.since; v <= SchemaVersion; v++ {
			if _, err := os.Stat(goldenPath(tc.name, v)); err != nil {
				t.Errorf("golden file for %s at version %d is missing: earlier versions must be kept", tc.name, v)
			}
		}
	}
}
