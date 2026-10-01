package cli

// SchemaVersion is the version of the JSON shape of every command's output.
// Any change to a shape needs a new version and a new golden file.
const SchemaVersion = 1

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
