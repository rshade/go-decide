package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/kataras/jev"
	ax "github.com/rshade/ax-go"
	"github.com/rshade/ax-go/contract"
	"github.com/spf13/cobra"

	"github.com/rshade/go-decide/decision"
	"github.com/rshade/go-decide/eval"
	"github.com/rshade/go-decide/jevclient"
)

func newEvalCommand(env Env) *cobra.Command {
	var in inputFlags
	var decisionsPath, truthPath, cacheDir string

	cmd := &cobra.Command{
		Use:   "eval",
		Short: "Measure how well Jev's confidence separates clear decisions from contested ones",
		Long: "eval asks Jev one choice question per decision in a labelled set and reports\n" +
			"accuracy, contested AUC, Brier score and precision and recall per threshold.\n" +
			"It measures; it never approves. Both files are validated before any request.\n" +
			fmt.Sprintf("Output schema_version: %d. Exit code 0 when the report is printed.", SchemaVersion),
		Example: `  go-decide eval --decisions testdata/decisions.json --truth testdata/decisions_truth.json
  go-decide eval --decisions past.json --truth past_truth.json --cache-dir .eval-cache`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			thresholds, err := in.thresholds(cmd)
			if err != nil {
				return err
			}
			items, err := loadEvalSet(ctx, decisionsPath, truthPath, in.instructions)
			if err != nil {
				return err
			}
			if contract.DryRunFromContext(ctx) {
				return writeDryRun(cmd, evalDryRunOutput(items, thresholds))
			}

			var opts []jevclient.Option
			if cacheDir != "" {
				opts = append(opts, jevclient.WithResponseCache(cacheDir))
			}
			client, err := env.NewClient(opts...)
			if err != nil {
				return jevclient.Classify(ctx, err)
			}
			results, err := askAll(ctx, client, items, thresholds)
			if err != nil {
				return err
			}
			report, err := eval.Compute(results, thresholds)
			if err != nil {
				return err
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(ctx, evalOutput(items, results, report, thresholds)))
		},
	}
	cmd.Flags().StringVar(&decisionsPath, "decisions", "", "the decision set, as a JSON file in the format of testdata/decisions.json")
	cmd.Flags().StringVar(&truthPath, "truth", "", "the truth file, as a JSON file in the format of testdata/decisions_truth.json")
	cmd.Flags().StringVar(&in.instructions, "instructions", defaultEvalInstructions, "what to decide, asked of every decision")
	in.registerThresholds(cmd)
	cmd.Flags().StringVar(&cacheDir, "cache-dir", "", "store successful responses here so a rerun of the same set is free")
	ax.WithNonDeterministicFields[EvalOutput](cmd)
	return cmd
}

// askAll asks every question in order and stops at the first failure, so no
// partial report exists. Responses before the failure stay in the cache.
func askAll(ctx context.Context, client *jev.Client, items []evalItem, th decision.Thresholds) ([]eval.Result, error) {
	results := make([]eval.Result, 0, len(items))
	for _, item := range items {
		answer, err := decision.Choose(ctx, client, item.question, decision.WithThresholds(th))
		if err != nil {
			return nil, withDecision(ctx, item.id, failure(ctx, err))
		}
		r, err := eval.NewResult(item.class, answer.Leading(), answer.Confidence(), item.correct)
		if err != nil {
			return nil, withDecision(ctx, item.id, err)
		}
		results = append(results, r)
	}
	return results, nil
}

// withDecision names the decision a failure belongs to, keeping its code.
func withDecision(ctx context.Context, id string, err error) error {
	var ce *contract.Error
	if !errors.As(err, &ce) {
		return contract.NewError(ctx, "internal", fmt.Sprintf("decision %s: %v", id, err),
			contract.WithErrorContext(map[string]any{"decision": id}))
	}
	ce.Message = fmt.Sprintf("decision %s: %s", id, ce.Message)
	if ce.Context == nil {
		ce.Context = map[string]any{}
	}
	ce.Context["decision"] = id
	return err
}

func evalDryRunOutput(items []evalItem, th decision.Thresholds) EvalDryRunOutput {
	out := EvalDryRunOutput{SchemaVersion: SchemaVersion, DryRun: true, Decisions: len(items), Thresholds: thresholdsOutput(th)}
	for _, item := range items {
		if item.class == eval.Dominant {
			out.Dominant++
		} else {
			out.Contested++
		}
	}
	return out
}

func evalOutput(items []evalItem, results []eval.Result, report eval.Report, th decision.Thresholds) EvalOutput {
	out := EvalOutput{
		SchemaVersion: SchemaVersion,
		Metrics: EvalMetricsOutput{
			Accuracy:     metricOutput(report.Accuracy),
			ContestedAUC: metricOutput(report.ContestedAUC),
			Brier:        report.Brier,
		},
		Outcomes: EvalOutcomesOutput{
			Decided:       report.Outcomes.Decided,
			Uncertain:     report.Outcomes.Uncertain,
			Escalate:      report.Outcomes.Escalate,
			DecidedUnsafe: report.Outcomes.DecidedUnsafe,
		},
		Thresholds:  thresholdsOutput(th),
		ByThreshold: make([]EvalThresholdRow, len(report.ByThreshold)),
		Decisions:   make([]EvalDecisionRow, len(results)),
	}
	for i, row := range report.ByThreshold {
		out.ByThreshold[i] = EvalThresholdRow{
			Threshold: row.Threshold.Float64(),
			Selected:  row.Selected,
			Safe:      row.Safe,
			Unsafe:    row.Unsafe,
			Precision: metricOutput(row.Precision),
			Recall:    metricOutput(row.Recall),
		}
	}
	for i, r := range results {
		row := EvalDecisionRow{
			ID:         items[i].id,
			Class:      r.Class().String(),
			Pick:       r.Pick(),
			Confidence: r.Confidence().Float64(),
			Safe:       r.Safe(),
			Outcome:    eval.OutcomeOf(r.Confidence(), th).String(),
		}
		if c := r.Correct(); c != "" {
			row.CorrectOption = &c
		}
		out.Decisions[i] = row
	}
	return out
}

func metricOutput(m eval.Metric) *float64 {
	v, ok := m.Get()
	if !ok {
		return nil
	}
	return &v
}
