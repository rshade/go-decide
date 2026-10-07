package cli

import (
	"fmt"

	ax "github.com/rshade/ax-go"
	"github.com/rshade/ax-go/contract"
	"github.com/spf13/cobra"

	"github.com/rshade/go-decide/decision"
)

func newAskCommand(env Env, outcome *outcomeKind) *cobra.Command {
	var in inputFlags
	var options []string

	cmd := &cobra.Command{
		Use:   "ask",
		Short: "Ask a System One model to choose between options",
		Long: "ask puts a choice question to the --backend model (Jev by default) and prints the outcome as a JSON envelope.\n" +
			"Only a decided outcome has a choice; the other two name a leading option only.\n" +
			fmt.Sprintf("Output schema_version: %d. %s.", SchemaVersion, outcomeExitCodes),
		Example: `  go-decide ask --spec decision.json
  go-decide ask --state "all tests passed" --instructions "Ship it?" --option ship="safe to release" --option hold=wait`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			b, err := in.backend.resolve(ctx)
			if err != nil {
				return err
			}
			entries, err := parseEntries(ctx, "option", options)
			if err != nil {
				return err
			}
			spec, err := in.loadSpec(cmd, entries, false)
			if err != nil {
				return err
			}
			question, err := spec.Question()
			if err != nil {
				return failure(ctx, err)
			}
			thresholds, err := in.thresholds(cmd, b)
			if err != nil {
				return err
			}
			if contract.DryRunFromContext(ctx) {
				return writeDryRun(cmd, dryRunOutput(b.name, "choice", spec.Options, thresholds))
			}
			client, err := b.client(ctx, env, "")
			if err != nil {
				return err
			}

			result, err := decision.Choose(ctx, client, question, b.decisionOptions(thresholds)...)
			if err != nil {
				return failure(ctx, err)
			}

			out, kind := askOutput(b.name, result, thresholds)
			*outcome = kind
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(ctx, out))
		},
	}
	in.register(cmd)
	cmd.Flags().StringArrayVar(&options, "option", nil, "an option as name=description, repeated; the order is kept")
	ax.WithNonDeterministicFields[AskOutput](cmd)
	return cmd
}

func askOutput(backend string, result decision.Result[string], thresholds decision.Thresholds) (AskOutput, outcomeKind) {
	out := AskOutput{
		SchemaVersion: SchemaVersion,
		Backend:       backend,
		Confidence:    result.Confidence().Float64(),
		Probabilities: probabilitiesOutput(result.Probabilities()),
		Thresholds:    thresholdsOutput(thresholds),
	}
	kind := decision.Match(result,
		func(d decision.Decided[string]) outcomeKind {
			out.Choice = d.Choice()
			return outcomeDecided
		},
		func(u decision.Uncertain[string]) outcomeKind {
			out.Leading = u.Leading()
			return outcomeUncertain
		},
		func(e decision.Escalate[string]) outcomeKind {
			out.Leading = e.Leading()
			return outcomeEscalate
		},
	)
	out.Outcome = kind.name()
	return out, kind
}
