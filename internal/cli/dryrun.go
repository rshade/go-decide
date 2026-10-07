package cli

import (
	ax "github.com/rshade/ax-go"
	"github.com/spf13/cobra"
)

// dryRunIdempotencyKey is what a dry run prints when --idempotency-key was
// not set. A dry run sends nothing, so the key is a constant and two runs
// of the same command print the same JSON.
const dryRunIdempotencyKey = "dry-run"

func writeDryRun[T any](cmd *cobra.Command, data T) error {
	env := ax.NewEnvelope(cmd.Context(), data)
	env.Meta.TraceID = ax.ZeroTraceID
	env.Meta.SpanID = ax.ZeroSpanID
	env.Meta.DryRun = true
	if !idempotencyKeySet(cmd) {
		env.Meta.IdempotencyKey = dryRunIdempotencyKey
	}
	return ax.WriteJSON(cmd.OutOrStdout(), env)
}

func idempotencyKeySet(cmd *cobra.Command) bool {
	if cmd.Flags().Changed("idempotency-key") {
		return true
	}
	return cmd.InheritedFlags().Changed("idempotency-key")
}
