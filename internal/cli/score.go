package cli

import (
	"fmt"
	"sync/atomic"

	ax "github.com/rshade/ax-go"
	"github.com/rshade/ax-go/contract"
	"github.com/spf13/cobra"

	"github.com/rshade/go-decide/decision"
)

func newScoreCommand(env Env, outcome *outcomeRecorder, serving *atomic.Bool) *cobra.Command {
	in := inputFlags{serving: serving}
	var levels []string

	cmd := &cobra.Command{
		Use:   "score",
		Short: "Ask Jev to rate content against an ordered rubric",
		Long: "score rates the state against ordered levels and prints the outcome as a JSON envelope.\n" +
			"Only a decided outcome has a level; the other two name the nearest level only.\n" +
			fmt.Sprintf("Output schema_version: %d. %s.", SchemaVersion, outcomeExitCodes),
		Example: `  go-decide score --spec incident.json
  go-decide score --state "checkout fails for half of users" --level minor="cosmetic" --level major="cannot buy"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			b, err := in.backend.resolve(ctx)
			if err != nil {
				return err
			}
			entries, err := parseEntries(ctx, "level", levels)
			if err != nil {
				return err
			}
			spec, err := in.loadSpec(cmd, entries, true)
			if err != nil {
				return err
			}
			if err := b.checkInstructions(ctx, spec.Instructions); err != nil {
				return err
			}
			question, err := spec.RateQuestion()
			if err != nil {
				return failure(ctx, err)
			}
			thresholds, err := in.thresholds(cmd, b)
			if err != nil {
				return err
			}
			if contract.DryRunFromContext(ctx) {
				return writeDryRun(cmd, dryRunOutput(b.name, "score", spec.Levels, thresholds))
			}
			client, err := b.client(ctx, env, "")
			if err != nil {
				return err
			}

			result, err := decision.Rate(ctx, client, question, b.decisionOptions(thresholds)...)
			if err != nil {
				return failure(ctx, err)
			}

			out, kind := scoreOutput(b.name, result, thresholds)
			outcome.record(kind)
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(ctx, out))
		},
	}
	in.register(cmd)
	cmd.Flags().StringArrayVar(&levels, "level", nil, "a level as name=description, repeated, lowest first")
	ax.WithNonDeterministicFields[ScoreOutput](cmd)
	return cmd
}

func scoreOutput(backend string, result decision.ScoreResult[string], thresholds decision.Thresholds) (ScoreOutput, outcomeKind) {
	out := ScoreOutput{
		SchemaVersion: SchemaVersion,
		Backend:       backend,
		Score:         result.Score(),
		Confidence:    result.Confidence().Float64(),
		Probabilities: probabilitiesOutput(result.Probabilities()),
		Thresholds:    thresholdsOutput(thresholds),
	}
	kind := decision.MatchScore(result,
		func(d decision.ScoreDecided[string]) outcomeKind {
			out.Level = d.Level()
			return outcomeDecided
		},
		func(u decision.ScoreUncertain[string]) outcomeKind {
			out.Nearest = u.Nearest()
			return outcomeUncertain
		},
		func(e decision.ScoreEscalate[string]) outcomeKind {
			out.Nearest = e.Nearest()
			return outcomeEscalate
		},
	)
	out.Outcome = kind.name()
	return out, kind
}
