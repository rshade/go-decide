// Package cli is the go-decide command line: the ask and score commands on
// ax-go, with versioned JSON output and an exit code per outcome.
package cli

import (
	"context"
	"io"
	"os"

	"github.com/kataras/jev"
	ax "github.com/rshade/ax-go"
	"github.com/spf13/cobra"

	"github.com/rshade/go-decide/jevclient"
)

// Exit codes for an outcome that is not a failure. Failures use the shared
// ax-go codes. Code 0 is a decided outcome.
const (
	ExitUncertain = 10
	ExitEscalate  = 11
)

// Env is what a run needs from the outside. The zero value of each field falls
// back to the process: os.Stdin, os.Stdout, os.Stderr, os.Getenv, the resolved
// build version and a client built from the environment.
type Env struct {
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	Getenv    func(string) string
	Version   string
	NewClient func(...jevclient.Option) (*jev.Client, error)
}

func (e Env) withDefaults() Env {
	if e.Stdin == nil {
		e.Stdin = os.Stdin
	}
	if e.Stdout == nil {
		e.Stdout = os.Stdout
	}
	if e.Stderr == nil {
		e.Stderr = os.Stderr
	}
	if e.Getenv == nil {
		e.Getenv = os.Getenv
	}
	if e.NewClient == nil {
		e.NewClient = jevclient.NewClient
	}
	e.Version = ax.ResolveVersion(e.Version)
	return e
}

// Run executes the command line in args and returns the process exit code. It
// prints a result on standard output and a failure as an error envelope on
// standard error.
func Run(ctx context.Context, args []string, env Env) int {
	env = env.withDefaults()
	var outcome outcomeKind
	root := newRoot(env, &outcome)
	root.SetArgs(args)

	code := ax.Execute(ctx, root,
		ax.WithStdin(env.Stdin),
		ax.WithStdout(env.Stdout),
		ax.WithStderr(env.Stderr),
		ax.WithEnv(env.Getenv),
		ax.WithVersion(env.Version),
	)
	if code != ax.ExitSuccess {
		return code
	}
	return outcome.exitCode()
}

func newRoot(env Env, outcome *outcomeKind) *cobra.Command {
	root := &cobra.Command{
		Use:     "go-decide",
		Version: env.Version,
		Short:   "Ask a System One model a choice or score question and get a typed outcome",
		Long: "go-decide asks a System One model, Jev by default: a choice (ask) or an ordered rubric (score).\n" +
			"The outcome is decided, uncertain or escalate, and only a decided outcome is one to act on.\n" +
			"eval measures how well the confidence separates clear decisions from contested ones.\n" +
			"The default thresholds (floor 0.5, confident 0.9) are placeholders until #6.\n" +
			outcomeExitCodes + ", 1 to 4 failures; eval exits 0 with a report.",
	}
	// Print only the release version, so --version matches the ldflags value.
	root.SetVersionTemplate("{{.Version}}\n")
	root.AddCommand(
		newAskCommand(env, outcome),
		newScoreCommand(env, outcome),
		newEvalCommand(env),
		newSchemaCommand(root),
	)
	return root
}
