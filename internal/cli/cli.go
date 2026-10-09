// Package cli is the go-decide command line: the ask and score commands on
// ax-go, with versioned JSON output and an exit code per outcome.
package cli

import (
	"context"
	"io"
	"os"
	"sync/atomic"

	"github.com/kataras/jev"
	ax "github.com/rshade/ax-go"
	"github.com/rshade/ax-go/mcp"
	"github.com/spf13/cobra"

	"github.com/rshade/go-decide/clefclient"
	"github.com/rshade/go-decide/jevclient"
)

// Exit codes for an outcome that is not a failure. Failures use the shared
// ax-go codes. Code 0 is a decided outcome.
const (
	ExitUncertain = 10
	ExitEscalate  = 11
)

const mcpServerName = "mcp-server"

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
	// NewClefClient builds the client for --backend clef.
	NewClefClient func(...clefclient.Option) (*jev.Client, error)
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
	if e.NewClefClient == nil {
		e.NewClefClient = clefclient.NewClient
	}
	e.Version = ax.ResolveVersion(e.Version)
	return e
}

// Run executes the command line in args and returns the process exit code. It
// prints a result on standard output and a failure as an error envelope on
// standard error.
func Run(ctx context.Context, args []string, env Env) int {
	env = env.withDefaults()
	var outcome outcomeRecorder
	root, err := newRoot(env, &outcome)
	if err != nil {
		_ = ax.WriteError(env.Stderr, ax.NewError(ctx, "internal_error", err.Error(),
			ax.WithErrorExitCode(ax.ExitInternal)))
		return ax.ExitInternal
	}
	root.SetArgs(args)
	serving := isMCPServer(root, args)

	code := ax.Execute(ctx, root,
		ax.WithStdin(env.Stdin),
		ax.WithStdout(env.Stdout),
		ax.WithStderr(env.Stderr),
		ax.WithEnv(env.Getenv),
		ax.WithVersion(env.Version),
	)
	if code != ax.ExitSuccess || serving {
		return code
	}
	return outcome.recorded().exitCode()
}

// isMCPServer reports whether args run the mcp-server command. That process
// ends with 0 or a failure code: an outcome a tool call produced is a result,
// never the exit code of the server.
func isMCPServer(root *cobra.Command, args []string) bool {
	cmd, _, err := root.Find(args)
	return err == nil && cmd.Name() == mcpServerName
}

func newRoot(env Env, outcome *outcomeRecorder) (*cobra.Command, error) {
	root := &cobra.Command{
		Use:     "go-decide",
		Version: env.Version,
		Short:   "Ask a System One model a choice or score question and get a typed outcome",
		Long: "go-decide asks a System One model, Jev by default or clef with --backend clef: a choice (ask) or an ordered rubric (score).\n" +
			"The outcome is decided, uncertain or escalate, and only a decided outcome is one to act on.\n" +
			"eval measures how well the confidence separates clear decisions from contested ones.\n" +
			"The default thresholds (floor 0.5, confident 0.9, the same for each backend) are placeholders until #6.\n" +
			outcomeExitCodes + ", 1 to 4 failures; eval exits 0 with a report.",
	}
	// Print only the release version, so --version matches the ldflags value.
	root.SetVersionTemplate("{{.Version}}\n")
	instructions, err := declareSkill(root)
	if err != nil {
		return nil, err
	}
	var serving atomic.Bool
	eval := newEvalCommand(env)
	mcp.Exclude(eval)
	server := mcp.NewCommand(root, mcp.WithVersion(env.Version), mcp.WithInstructions(instructions))
	serve := server.RunE
	server.RunE = func(cmd *cobra.Command, args []string) error {
		serving.Store(true)
		return serve(cmd, args)
	}
	root.AddCommand(
		newAskCommand(env, outcome, &serving),
		newScoreCommand(env, outcome, &serving),
		eval,
		newSchemaCommand(root),
		server,
	)
	return root, nil
}
