package cli

import (
	ax "github.com/rshade/ax-go"
	"github.com/rshade/ax-go/contract"
	"github.com/spf13/cobra"

	"github.com/rshade/jev-decide/decision"
	"github.com/rshade/jev-decide/jevclient"
)

func newScoreCommand(env Env, outcome *outcomeKind) *cobra.Command {
	var in inputFlags
	var levels []string

	cmd := &cobra.Command{
		Use:   "score",
		Short: "Ask Jev to rate content against an ordered rubric",
		Long: "score rates the state against ordered levels and prints the outcome as a JSON envelope.\n" +
			"Only a decided outcome has a level; the other two name the nearest level only.\n" +
			"Output schema_version: 1. Exit codes: 0 decided, 10 uncertain, 11 escalate.",
		Example: `  jev-decide score --spec incident.json
  jev-decide score --state "checkout fails for half of users" --level minor="cosmetic" --level major="cannot buy"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			entries, err := parseEntries(ctx, "level", levels)
			if err != nil {
				return err
			}
			spec, err := in.loadSpec(cmd, entries, true)
			if err != nil {
				return err
			}
			question, err := spec.RateQuestion()
			if err != nil {
				return failure(ctx, err)
			}
			thresholds, err := in.thresholds(cmd)
			if err != nil {
				return err
			}
			if contract.DryRunFromContext(ctx) {
				return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(ctx, dryRunOutput("score", spec.Levels, thresholds)))
			}
			client, err := env.NewClient()
			if err != nil {
				return jevclient.Classify(ctx, err)
			}

			result, err := decision.Rate(ctx, client, question, decision.WithThresholds(thresholds))
			if err != nil {
				return failure(ctx, err)
			}

			out, kind := scoreOutput(result, thresholds)
			*outcome = kind
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(ctx, out))
		},
	}
	in.register(cmd)
	cmd.Flags().StringArrayVar(&levels, "level", nil, "a level as name=description, repeated, lowest first")
	ax.WithNonDeterministicFields[ScoreOutput](cmd)
	return cmd
}

func scoreOutput(result decision.ScoreResult[string], thresholds decision.Thresholds) (ScoreOutput, outcomeKind) {
	out := ScoreOutput{
		SchemaVersion: SchemaVersion,
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
