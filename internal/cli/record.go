package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	ax "github.com/rshade/ax-go"
	"github.com/rshade/ax-go/contract"
	"github.com/spf13/cobra"

	"github.com/rshade/go-decide/eval"
)

const (
	pathFast    = "fast"
	pathDebate  = "debate"
	pathStopped = "stopped"
)

const defaultPendingDays = 30

const recordOff = "Recording is off unless DECIDE_LOG names the log file; then nothing is read or written and the result says recording is false."

var recordExitCode = fmt.Sprintf("Output schema_version: %d. Exit code 0 when the result is printed.", SchemaVersion)

// debateRecord is what a debate run adds to its run line.
type debateRecord struct {
	Recommendation string           `json:"recommendation"`
	EvidenceMode   string           `json:"evidence_mode,omitempty"`
	GoDecideCalls  int              `json:"go_decide_calls"`
	Citations      []citationRecord `json:"citations"`
}

// citationRecord is one go-decide result a debate agent cited, with what it
// asked, so a reader can see which positions the question left out.
// The measured fields are pointers so a citation that leaves one out is
// refused rather than recorded as a zero the model never reported.
type citationRecord struct {
	Agent      string        `json:"agent"`
	Round      int           `json:"round"`
	Backend    string        `json:"backend"`
	Outcome    string        `json:"outcome"`
	Result     string        `json:"result"`
	Confidence *float64      `json:"confidence"`
	Asked      string        `json:"asked"`
	Options    []citedOption `json:"options"`
}

type citedOption struct {
	Name        string   `json:"name"`
	Probability *float64 `json:"probability"`
}

// preScreenFields are the keys ask always prints in its envelope data.
var preScreenFields = []string{"schema_version", "backend", "outcome", "confidence", "probabilities", "thresholds"}

func newRecordCommand(env Env) *cobra.Command {
	rec := newRecorder(env)
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Keep an opt-in log of decisions, their runs and their outcomes",
		Long: "record keeps the decision log the decide skill writes: each decision before its answer is known,\n" +
			"the path its run took, and the outcome the operator labels later. eval --log scores the labelled ones.\n" +
			recordOff,
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(
		newRecordDecisionCommand(rec),
		newRecordRunCommand(rec),
		newRecordOutcomeCommand(rec),
		newRecordPendingCommand(rec),
	)
	return cmd
}

func recordLong(what string) string {
	return what + "\n" + recordOff + "\n" +
		"A log inside a git working tree is written only with --yes. " + recordExitCode
}

func writeRecord(cmd *cobra.Command, out RecordOutput) error {
	out.SchemaVersion = SchemaVersion
	return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), out))
}

// writeRecordOff prints the result of a record command while recording is off.
func writeRecordOff(cmd *cobra.Command, kind string) error {
	if contract.DryRunFromContext(cmd.Context()) {
		return writeDryRun(cmd, RecordOutput{SchemaVersion: SchemaVersion, DryRun: true, Kind: kind})
	}
	return writeRecord(cmd, RecordOutput{Kind: kind})
}

// commitRecord appends the line build produces, or under --dry-run checks it
// against the log and writes nothing, and prints the result.
func commitRecord(cmd *cobra.Command, rec *recorder, kind string, build func([]logLine) (logLine, error)) error {
	ctx := cmd.Context()
	if contract.DryRunFromContext(ctx) {
		if err := rec.validate(ctx, build); err != nil {
			return err
		}
		return writeDryRun(cmd, RecordOutput{SchemaVersion: SchemaVersion, Recording: true, DryRun: true, Kind: kind, Log: rec.path()})
	}
	line, err := rec.commit(ctx, build)
	if err != nil {
		return err
	}
	return writeRecord(cmd, RecordOutput{Recording: true, Kind: kind, ID: line.ID, Log: rec.path()})
}

func newRecordDecisionCommand(rec *recorder) *cobra.Command {
	var title, contextText, instructions string
	var constraints, options []string

	cmd := &cobra.Command{
		Use:   "decision",
		Short: "Record a decision before the pre-screen or the debate answers it",
		Long:  recordLong("record decision appends the decision as the pre-screen asks it, and prints its new id."),
		Example: `  go-decide record decision --title "Cache backend" --context "Reads dominate." \
    --instructions "Which cache should the service use?" --option postgres="Unlogged table" --option sqlite="Local file"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if !rec.on() {
				return writeRecordOff(cmd, kindDecision)
			}
			d, err := recordedDecision(ctx, title, contextText, instructions, constraints, options)
			if err != nil {
				return err
			}
			return commitRecord(cmd, rec, kindDecision, func(lines []logLine) (logLine, error) {
				id, err := rec.newDecisionID(lines)
				if err != nil {
					return logLine{}, writeFailure(ctx, err)
				}
				d.ID = id
				raw, err := json.Marshal(d)
				if err != nil {
					return logLine{}, writeFailure(ctx, err)
				}
				return logLine{Kind: kindDecision, ID: id, Instructions: instructions, Decision: raw}, nil
			})
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "the decision in one line")
	cmd.Flags().StringVar(&contextText, "context", "", "the context gathered before any answer")
	cmd.Flags().StringArrayVar(&constraints, "constraint", nil, "a hard constraint, repeated")
	cmd.Flags().StringVar(&instructions, "instructions", "", "the question the pre-screen asks")
	cmd.Flags().StringArrayVar(&options, "option", nil, "an option as name=description, repeated; the order is kept")
	ax.WithNonDeterministicFields[RecordOutput](cmd)
	return cmd
}

// recordedDecision builds the decision entry and checks it by the rules eval
// applies, so every recorded decision is one eval --log accepts.
func recordedDecision(ctx context.Context, title, contextText, instructions string, constraints, options []string) (evalDecision, error) {
	if strings.TrimSpace(title) == "" {
		return evalDecision{}, newValidationError(ctx, "title is required", "title")
	}
	if strings.TrimSpace(instructions) == "" {
		return evalDecision{}, newValidationError(ctx, "instructions is required", "instructions")
	}
	entries, err := parseEntries(ctx, "option", options)
	if err != nil {
		return evalDecision{}, err
	}
	d := evalDecision{Title: title, Context: contextText, Constraints: constraints, Options: make([]json.RawMessage, len(entries))}
	if d.Constraints == nil {
		d.Constraints = []string{}
	}
	for i, e := range entries {
		raw, err := json.Marshal(map[string]string{"name": e.Name, "description": e.Description})
		if err != nil {
			return evalDecision{}, err
		}
		d.Options[i] = raw
	}
	if _, err := decisionQuestion(ctx, d, entries, instructions, recordField); err != nil {
		return evalDecision{}, err
	}
	return d, nil
}

// recordField names the record flag behind a part of the decision. sub is a
// decision.FieldError field ("Options", "State") or an option path from
// optionEntries ("options[0].name").
func recordField(sub string) string {
	sub = strings.ToLower(sub)
	switch {
	case strings.HasPrefix(sub, "options"):
		return "option"
	case sub == "state":
		return "context"
	default:
		return sub
	}
}

func newRecordRunCommand(rec *recorder) *cobra.Command {
	var id, path, preScreen, preScreenError, preScreenSkipped, recommendation, evidenceMode string
	var calls int
	var citations []string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Record the path a decision's run took, when the run ends",
		Long: recordLong("record run appends the run of a recorded decision: --path fast (the pre-screen decided),\n" +
			"debate, or stopped (the user stopped after an uncertain result or a failed call), with exactly one of\n" +
			"--pre-screen, --pre-screen-error and --pre-screen-skipped. A debate also takes the recommendation,\n" +
			"the evidence mode, the debate agents' go-decide calls and each citation."),
		Example: `  go-decide record run --id 2026-10-12-a1b2 --path stopped --pre-screen-error jev.timeout`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if !rec.on() {
				return writeRecordOff(cmd, kindRun)
			}
			if id == "" {
				return newValidationError(ctx, "id is required", "id")
			}
			pre, outcome, err := runPreScreen(ctx, preScreen, preScreenError, preScreenSkipped)
			if err != nil {
				return err
			}
			if err := checkRunPath(ctx, path, outcome, preScreenError != ""); err != nil {
				return err
			}
			debate, err := runDebate(ctx, cmd, path, recommendation, evidenceMode, calls, citations)
			if err != nil {
				return err
			}
			return commitRecord(cmd, rec, kindRun, func(lines []logLine) (logLine, error) {
				if _, err := decisionFor(ctx, lines, id); err != nil {
					return logLine{}, err
				}
				return logLine{Kind: kindRun, ID: id, Path: path, PreScreen: pre, Debate: debate}, nil
			})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "the id record decision returned")
	cmd.Flags().StringVar(&path, "path", "", "fast, debate or stopped")
	cmd.Flags().StringVar(&preScreen, "pre-screen", "", "the ask envelope's data: a JSON object, passed as a string")
	cmd.Flags().StringVar(&preScreenError, "pre-screen-error", "", "the error code of a failed pre-screen call")
	cmd.Flags().StringVar(&preScreenSkipped, "pre-screen-skipped", "", "why the pre-screen did not run")
	cmd.Flags().StringVar(&recommendation, "recommendation", "", "debate only: the Moderator's recommendation")
	cmd.Flags().StringVar(&evidenceMode, "evidence-mode", "", "debate only: blind or informed, when go-decide was available")
	cmd.Flags().IntVar(&calls, "go-decide-calls", 0, "debate only: the debate agents' go-decide calls")
	cmd.Flags().StringArrayVar(&citations, "citation", nil, "debate only: a cited go-decide result: a JSON object, passed as a string; repeated")
	ax.WithNonDeterministicFields[RecordOutput](cmd)
	return cmd
}

// runPreScreen returns the run line's pre_screen part and the outcome it
// carries ("" for a failure or a skip). Exactly one source must be given.
func runPreScreen(ctx context.Context, data, code, skipped string) (json.RawMessage, string, error) {
	given := 0
	for _, v := range []string{data, code, skipped} {
		if v != "" {
			given++
		}
	}
	if given != 1 {
		return nil, "", newValidationError(ctx, "give exactly one of pre-screen, pre-screen-error and pre-screen-skipped", "pre-screen")
	}
	var part map[string]string
	switch {
	case code != "":
		part = map[string]string{"error": code}
	case skipped != "":
		part = map[string]string{"skipped": skipped}
	default:
		raw, outcome, err := parsePreScreen(data)
		if err != nil {
			return nil, "", newValidationError(ctx, "pre-screen "+err.Error(), "pre-screen")
		}
		return raw, outcome, nil
	}
	raw, err := json.Marshal(part)
	return raw, "", err
}

// parsePreScreen checks that data is an ask envelope's data with every field
// ask prints, and returns it as given, compacted. Re-marshalling the decoded
// value would write a field the caller left out as a zero.
func parsePreScreen(data string) (json.RawMessage, string, error) {
	var out AskOutput
	if err := decodeStrict(data, &out); err != nil {
		return nil, "", fmt.Errorf("is not an ask envelope's data: %w", err)
	}
	var fields, thresholds map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &fields); err != nil {
		return nil, "", fmt.Errorf("is not a JSON object: %w", err)
	}
	if name := absentField(fields, preScreenFields...); name != "" {
		return nil, "", fmt.Errorf("has no %s; pass the ask envelope's data whole", name)
	}
	if err := json.Unmarshal(fields["thresholds"], &thresholds); err != nil {
		return nil, "", fmt.Errorf("thresholds is not a JSON object: %w", err)
	}
	if name := absentField(thresholds, "floor", "confident"); name != "" {
		return nil, "", fmt.Errorf("has no thresholds.%s; pass the ask envelope's data whole", name)
	}
	switch {
	case len(out.Probabilities) == 0:
		return nil, "", fmt.Errorf("has no probabilities; pass the ask envelope's data whole")
	case out.Outcome == "decided" && out.Choice != "":
	case (out.Outcome == "uncertain" || out.Outcome == "escalate") && out.Leading != "":
	default:
		return nil, "", fmt.Errorf("needs outcome decided with a choice, or uncertain or escalate with a leading option")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(data)); err != nil {
		return nil, "", err
	}
	return compact.Bytes(), out.Outcome, nil
}

// absentField returns the first of names that fields lacks or holds as null,
// or "" when every one is present.
func absentField(fields map[string]json.RawMessage, names ...string) string {
	for _, name := range names {
		if v, ok := fields[name]; !ok || string(v) == "null" {
			return name
		}
	}
	return ""
}

func checkRunPath(ctx context.Context, path, outcome string, failed bool) error {
	switch path {
	case pathFast:
		if outcome != "decided" {
			return newValidationError(ctx, "a fast run needs a decided pre-screen", "path")
		}
	case pathStopped:
		if outcome != "uncertain" && !failed {
			return newValidationError(ctx, "a stopped run follows an uncertain result or a failed call", "path")
		}
	case pathDebate:
	default:
		return newValidationError(ctx, fmt.Sprintf("path %q is not fast, debate or stopped", path), "path")
	}
	return nil
}

func runDebate(ctx context.Context, cmd *cobra.Command, path, recommendation, mode string, calls int, citations []string) (json.RawMessage, error) {
	if path != pathDebate {
		for _, name := range []string{"recommendation", "evidence-mode", "go-decide-calls", "citation"} {
			if cmd.Flags().Changed(name) {
				return nil, newValidationError(ctx, "only a debate run takes "+name, name)
			}
		}
		return nil, nil
	}
	switch {
	case strings.TrimSpace(recommendation) == "":
		return nil, newValidationError(ctx, "a debate run needs a recommendation", "recommendation")
	case mode != "" && mode != "blind" && mode != "informed":
		return nil, newValidationError(ctx, fmt.Sprintf("evidence-mode %q is not blind or informed", mode), "evidence-mode")
	case calls < 0:
		return nil, newValidationError(ctx, "go-decide-calls cannot be negative", "go-decide-calls")
	}
	debate := debateRecord{Recommendation: recommendation, EvidenceMode: mode, GoDecideCalls: calls, Citations: []citationRecord{}}
	for i, raw := range citations {
		c, err := parseCitation(raw)
		if err != nil {
			return nil, newValidationError(ctx, fmt.Sprintf("citation %d: %v", i+1, err), "citation")
		}
		debate.Citations = append(debate.Citations, c)
	}
	return json.Marshal(debate)
}

func parseCitation(raw string) (citationRecord, error) {
	var c citationRecord
	if err := decodeStrict(raw, &c); err != nil {
		return c, err
	}
	switch {
	case c.Agent != "advocate_a" && c.Agent != "advocate_b" && c.Agent != "moderator":
		return c, fmt.Errorf("agent %q is not advocate_a, advocate_b or moderator", c.Agent)
	case c.Round != 1 && c.Round != 2:
		return c, fmt.Errorf("round %d is not 1 or 2", c.Round)
	case c.Backend == "":
		return c, fmt.Errorf("backend is missing or empty")
	case c.Outcome != "decided" && c.Outcome != "uncertain" && c.Outcome != "escalate":
		return c, fmt.Errorf("outcome %q is not decided, uncertain or escalate", c.Outcome)
	case strings.TrimSpace(c.Result) == "":
		return c, fmt.Errorf("result is missing or empty: a citation names the choice or leading option")
	case c.Confidence == nil:
		return c, fmt.Errorf("confidence is missing")
	case strings.TrimSpace(c.Asked) == "":
		return c, fmt.Errorf("asked is missing or empty: a citation shows the instructions it sent")
	case len(c.Options) == 0:
		return c, fmt.Errorf("options is missing or empty: a citation shows every option or level it sent")
	}
	for _, o := range c.Options {
		if o.Name == "" {
			return c, fmt.Errorf("an option has no name")
		}
		if o.Probability == nil {
			return c, fmt.Errorf("option %q has no probability", o.Name)
		}
	}
	return c, nil
}

// decodeStrict decodes one JSON object with unknown fields refused.
func decodeStrict(raw string, v any) error {
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return fmt.Errorf("more than one JSON value")
	}
	return nil
}

// decisionFor returns the first decision line for id, or a validation error
// naming the id when the log has none.
func decisionFor(ctx context.Context, lines []logLine, id string) (logLine, error) {
	for _, l := range lines {
		if l.Kind == kindDecision && l.ID == id {
			return l, nil
		}
	}
	return logLine{}, newValidationError(ctx, fmt.Sprintf("no decision %q in the log", id), "id")
}

func newRecordOutcomeCommand(rec *recorder) *cobra.Command {
	var id, class, correct, why string

	cmd := &cobra.Command{
		Use:   "outcome",
		Short: "Record how a decision turned out, as the operator judges it",
		Long: recordLong("record outcome appends an eval truth entry for a recorded decision: --class dominant with\n" +
			"the --correct-option that proved right, or contested with none, and --why. A later outcome for the\n" +
			"same id replaces an earlier one in eval --log; neither is ever rewritten."),
		Example: `  go-decide record outcome --id 2026-10-12-a1b2 --class dominant --correct-option postgres --why "It held up."`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if !rec.on() {
				return writeRecordOff(cmd, kindOutcome)
			}
			switch {
			case id == "":
				return newValidationError(ctx, "id is required", "id")
			case strings.TrimSpace(why) == "":
				return newValidationError(ctx, "why is required", "why")
			}
			cls, err := eval.ParseClass(class)
			if err != nil {
				return newValidationError(ctx, fmt.Sprintf("class %q is neither dominant nor contested", class), "class")
			}
			var given *string
			if correct != "" {
				given = &correct
			}
			return commitRecord(cmd, rec, kindOutcome, func(lines []logLine) (logLine, error) {
				line, err := decisionFor(ctx, lines, id)
				if err != nil {
					return logLine{}, err
				}
				var d evalDecision
				if err := json.Unmarshal(line.Decision, &d); err != nil {
					return logLine{}, newValidationError(ctx, fmt.Sprintf("%s: decision %q: %v", decideLogEnv, id, err), decideLogEnv)
				}
				entries, err := optionEntries(ctx, d, recordField)
				if err != nil {
					return logLine{}, err
				}
				if _, err := correctOption(ctx, "correct-option", cls, given, entries); err != nil {
					return logLine{}, err
				}
				raw, err := json.Marshal(evalTruth{ID: id, Class: class, CorrectOption: given, Why: why})
				if err != nil {
					return logLine{}, writeFailure(ctx, err)
				}
				return logLine{Kind: kindOutcome, ID: id, Truth: raw}, nil
			})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "the id record decision returned")
	cmd.Flags().StringVar(&class, "class", "", "dominant if one option proved right, contested if reasonable people would still disagree")
	cmd.Flags().StringVar(&correct, "correct-option", "", "the option that proved right; only for dominant")
	cmd.Flags().StringVar(&why, "why", "", "the operator's reason, in their words")
	ax.WithNonDeterministicFields[RecordOutput](cmd)
	return cmd
}

func newRecordPendingCommand(rec *recorder) *cobra.Command {
	days := defaultPendingDays

	cmd := &cobra.Command{
		Use:   "pending",
		Short: "List recorded decisions waiting for an outcome, without their runs",
		Long: "record pending lists the decisions recorded more than --older-than-days ago that have no outcome yet,\n" +
			"as they were recorded. It never shows a run, so a label is not anchored on the model's answer.\n" +
			recordOff + "\n" + recordExitCode,
		Example: `  go-decide record pending --older-than-days 30`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if days < 0 {
				return newValidationError(ctx, "older-than-days cannot be negative", "older-than-days")
			}
			out := PendingOutput{SchemaVersion: SchemaVersion, Recording: rec.on(), OlderThanDays: days, Decisions: []PendingDecision{}}
			if out.Recording {
				lines, err := rec.read()
				if err != nil {
					return logError(ctx, err)
				}
				if out.Decisions, err = pendingDecisions(lines, rec.env.Now().AddDate(0, 0, -days)); err != nil {
					return logError(ctx, err)
				}
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(ctx, out))
		},
	}
	cmd.Flags().IntVar(&days, "older-than-days", defaultPendingDays, "list only decisions recorded at least this many days ago")
	return cmd
}

// pendingDecisions returns, in log order, the first line of each decision
// recorded before cutoff that has no outcome line.
func pendingDecisions(lines []logLine, cutoff time.Time) ([]PendingDecision, error) {
	labelled := map[string]bool{}
	for _, l := range lines {
		if l.Kind == kindOutcome {
			labelled[l.ID] = true
		}
	}
	out := []PendingDecision{}
	seen := map[string]bool{}
	for _, l := range lines {
		if l.Kind != kindDecision || labelled[l.ID] || seen[l.ID] {
			continue
		}
		seen[l.ID] = true
		at, err := time.Parse(time.RFC3339, l.RecordedAt)
		if err != nil {
			return nil, fmt.Errorf("decision %q has recorded_at %q, not RFC 3339", l.ID, l.RecordedAt)
		}
		if at.Before(cutoff) {
			out = append(out, PendingDecision{ID: l.ID, RecordedAt: l.RecordedAt, Instructions: l.Instructions, Decision: l.Decision})
		}
	}
	return out, nil
}
