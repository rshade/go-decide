package cli

import "github.com/rshade/go-decide/decision"

// SchemaVersion is the version of the JSON shape of every command's output.
// Any change to a shape needs a new version and a new golden file.
const SchemaVersion = 3

// ThresholdsOutput echoes the thresholds a run used.
type ThresholdsOutput struct {
	Floor     float64 `json:"floor"`
	Confident float64 `json:"confident"`
}

// AskOutput is the data of the ask envelope. A decided outcome carries Choice;
// the other two carry Leading, so an option to act on is absent unless it is
// decided.
type AskOutput struct {
	SchemaVersion int                `json:"schema_version"`
	Outcome       string             `json:"outcome"`
	Choice        string             `json:"choice,omitempty"`
	Leading       string             `json:"leading,omitempty"`
	Confidence    float64            `json:"confidence"    ax:"nondeterministic"`
	Probabilities map[string]float64 `json:"probabilities" ax:"nondeterministic"`
	Thresholds    ThresholdsOutput   `json:"thresholds"`
}

// ScoreOutput is the data of the score envelope. A decided outcome carries
// Level; the other two carry Nearest.
type ScoreOutput struct {
	SchemaVersion int                `json:"schema_version"`
	Outcome       string             `json:"outcome"`
	Level         string             `json:"level,omitempty"`
	Nearest       string             `json:"nearest,omitempty"`
	Score         float64            `json:"score"         ax:"nondeterministic"`
	Confidence    float64            `json:"confidence"    ax:"nondeterministic"`
	Probabilities map[string]float64 `json:"probabilities" ax:"nondeterministic"`
	Thresholds    ThresholdsOutput   `json:"thresholds"`
}

// DryRunOutput is the data of the envelope a command prints under --dry-run: the
// spec and thresholds passed validation and nothing was asked. Names are the
// options of a choice or the levels of a score, in the order given.
type DryRunOutput struct {
	SchemaVersion int              `json:"schema_version"`
	DryRun        bool             `json:"dry_run"`
	Kind          string           `json:"kind"`
	Names         []string         `json:"names"`
	Thresholds    ThresholdsOutput `json:"thresholds"`
}

// EvalOutput is the data of the eval envelope: what the decision set measured
// under the thresholds of the run. A metric that is undefined for the set is
// null, never 0.
type EvalOutput struct {
	SchemaVersion int                `json:"schema_version"`
	Metrics       EvalMetricsOutput  `json:"metrics"      ax:"nondeterministic"`
	Outcomes      EvalOutcomesOutput `json:"outcomes"     ax:"nondeterministic"`
	Thresholds    ThresholdsOutput   `json:"thresholds"`
	ByThreshold   []EvalThresholdRow `json:"by_threshold" ax:"nondeterministic"`
	Decisions     []EvalDecisionRow  `json:"decisions"`
}

// EvalMetricsOutput holds the set-level metrics.
type EvalMetricsOutput struct {
	Accuracy     *float64 `json:"accuracy"`
	ContestedAUC *float64 `json:"contested_auc"`
	Brier        float64  `json:"brier"`
}

// EvalOutcomesOutput counts the outcomes under the run's thresholds.
type EvalOutcomesOutput struct {
	Decided       int `json:"decided"`
	Uncertain     int `json:"uncertain"`
	Escalate      int `json:"escalate"`
	DecidedUnsafe int `json:"decided_unsafe"`
}

// EvalThresholdRow is the fast path at one threshold.
type EvalThresholdRow struct {
	Threshold float64  `json:"threshold"`
	Selected  int      `json:"selected"`
	Safe      int      `json:"safe"`
	Unsafe    int      `json:"unsafe"`
	Precision *float64 `json:"precision"`
	Recall    *float64 `json:"recall"`
}

// EvalDecisionRow is one decision of the set. Pick is what Jev picked, which
// is a decision only when Outcome is decided.
type EvalDecisionRow struct {
	ID            string  `json:"id"`
	Class         string  `json:"class"`
	Pick          string  `json:"pick"           ax:"nondeterministic"`
	Confidence    float64 `json:"confidence"     ax:"nondeterministic"`
	CorrectOption *string `json:"correct_option"`
	Safe          bool    `json:"safe"           ax:"nondeterministic"`
	Outcome       string  `json:"outcome"        ax:"nondeterministic"`
}

// EvalDryRunOutput is the data of the envelope eval prints under --dry-run:
// both files and the thresholds passed validation and nothing was asked.
type EvalDryRunOutput struct {
	SchemaVersion int              `json:"schema_version"`
	DryRun        bool             `json:"dry_run"`
	Decisions     int              `json:"decisions"`
	Dominant      int              `json:"dominant"`
	Contested     int              `json:"contested"`
	Thresholds    ThresholdsOutput `json:"thresholds"`
}

func thresholdsOutput(th decision.Thresholds) ThresholdsOutput {
	return ThresholdsOutput{Floor: th.Floor().Float64(), Confident: th.Confident().Float64()}
}

func probabilitiesOutput(in map[string]decision.Probability) map[string]float64 {
	out := make(map[string]float64, len(in))
	for name, p := range in {
		out[name] = p.Float64()
	}
	return out
}

// dryRunOutput describes a validated question for --dry-run.
func dryRunOutput(kind string, entries []decision.SpecEntry, th decision.Thresholds) DryRunOutput {
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name
	}
	return DryRunOutput{SchemaVersion: SchemaVersion, DryRun: true, Kind: kind, Names: names, Thresholds: thresholdsOutput(th)}
}
