package decision_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildSnippet(t *testing.T, source string) (string, error) {
	t.Helper()
	if testing.Short() {
		t.Skip("compiles a program; skipped with -short")
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolving the module root: %v", err)
	}
	dir := t.TempDir()
	real := filepath.Join(dir, "main.go")
	if err := os.WriteFile(real, []byte(source), 0o600); err != nil {
		t.Fatalf("writing the snippet: %v", err)
	}
	overlay, err := json.Marshal(map[string]map[string]string{
		"Replace": {filepath.Join(root, "decision", "testdata", "snippet", "main.go"): real},
	})
	if err != nil {
		t.Fatalf("encoding the overlay: %v", err)
	}
	overlayFile := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(overlayFile, overlay, 0o600); err != nil {
		t.Fatalf("writing the overlay: %v", err)
	}

	cmd := exec.Command("go", "build", "-o", os.DevNull, "-overlay", overlayFile, "./decision/testdata/snippet")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	return string(out), err
}

const allHandlers = `package main

import "github.com/rshade/jev-decide/decision"

func main() {
	var r decision.Result[string]
	_ = decision.Match(r,
		func(decision.Decided[string]) int { return 1 },
		func(decision.Uncertain[string]) int { return 2 },
		func(decision.Escalate[string]) int { return 3 },
	)
}
`

const missingHandler = `package main

import "github.com/rshade/jev-decide/decision"

func main() {
	var r decision.Result[string]
	_ = decision.Match(r,
		func(decision.Decided[string]) int { return 1 },
		func(decision.Uncertain[string]) int { return 2 },
	)
}
`

const foreignResult = `package main

import "github.com/rshade/jev-decide/decision"

type fake struct{}

func (fake) Confidence() decision.Probability          { return decision.Probability{} }
func (fake) Leading() string                           { return "ship" }
func (fake) Probabilities() map[string]decision.Probability { return nil }
func (fake) Choice() string                            { return "ship" }

func main() {
	var _ decision.Decided[string] = fake{}
}
`

func TestCallersMustHandleAllThreeCases(t *testing.T) {
	if out, err := buildSnippet(t, allHandlers); err != nil {
		t.Fatalf("a program with all three handlers did not compile, so this check proves nothing: %v\n%s", err, out)
	}

	out, err := buildSnippet(t, missingHandler)

	if err == nil {
		t.Fatal("a program that omits a handler compiled, want a compile error")
	}
	if !strings.Contains(out, "not enough arguments in call to decision.Match") {
		t.Fatalf("build failed for another reason than the missing handler:\n%s", out)
	}
}

func TestNoCodeOutsideThePackageCanCreateAResult(t *testing.T) {
	out, err := buildSnippet(t, foreignResult)

	if err == nil {
		t.Fatal("a type from outside the package satisfied Decided, want a compile error")
	}
	if !strings.Contains(out, "does not implement decision.Decided[string]") || !strings.Contains(out, "isDecided") {
		t.Fatalf("build failed for another reason than the sealed method:\n%s", out)
	}
}

const scoreMissingHandler = `package main

import "github.com/rshade/jev-decide/decision"

func main() {
	var r decision.ScoreResult[string]
	_ = decision.MatchScore(r,
		func(decision.ScoreDecided[string]) int { return 1 },
		func(decision.ScoreUncertain[string]) int { return 2 },
	)
}
`

const scoreForeignResult = `package main

import "github.com/rshade/jev-decide/decision"

type fake struct{}

func (fake) Score() float64                                   { return 1 }
func (fake) Nearest() string                                  { return "high" }
func (fake) Confidence() decision.Probability                 { return decision.Probability{} }
func (fake) Probabilities() map[string]decision.Probability   { return nil }
func (fake) Level() string                                    { return "high" }

func main() {
	var _ decision.ScoreDecided[string] = fake{}
}
`

func TestCallersMustHandleAllThreeScoreCases(t *testing.T) {
	out, err := buildSnippet(t, scoreMissingHandler)

	if err == nil {
		t.Fatal("a program that omits a score handler compiled, want a compile error")
	}
	if !strings.Contains(out, "not enough arguments in call to decision.MatchScore") {
		t.Fatalf("build failed for another reason than the missing handler:\n%s", out)
	}
}

func TestNoCodeOutsideThePackageCanCreateAScoreResult(t *testing.T) {
	out, err := buildSnippet(t, scoreForeignResult)

	if err == nil {
		t.Fatal("a type from outside the package satisfied ScoreDecided, want a compile error")
	}
	if !strings.Contains(out, "does not implement decision.ScoreDecided[string]") || !strings.Contains(out, "isScoreDecided") {
		t.Fatalf("build failed for another reason than the sealed method:\n%s", out)
	}
}
