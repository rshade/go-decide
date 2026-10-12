package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/rshade/go-decide/decision"
	"github.com/rshade/go-decide/eval"
)

// maxEvalFileBytes bounds what eval reads from each input file.
const maxEvalFileBytes = 16 << 20

const defaultEvalInstructions = "Given the stated constraints, which option should the team choose?"

// evalDecision is one entry of the decision set. Options stay raw because they
// are content sent to Jev, so fields such as pros and costs pass through.
type evalDecision struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Context     string            `json:"context"`
	Constraints []string          `json:"constraints"`
	Options     []json.RawMessage `json:"options"`
}

// evalTruth is one entry of the truth file.
type evalTruth struct {
	ID            string  `json:"id"`
	Class         string  `json:"class"`
	CorrectOption *string `json:"correct_option"`
	Why           string  `json:"why"`
}

// evalItem is a validated decision, ready to ask.
type evalItem struct {
	id       string
	class    eval.Class
	correct  string
	question decision.Question[string]
}

// loadEvalSet reads and validates both files and builds one question per
// decision, in the order of the decision set. It sends nothing.
func loadEvalSet(ctx context.Context, decisionsPath, truthPath, instructions string) ([]evalItem, error) {
	decisions, err := readEntries[evalDecision](ctx, "decisions", decisionsPath)
	if err != nil {
		return nil, err
	}
	truths, err := readEntries[evalTruth](ctx, "truth", truthPath)
	if err != nil {
		return nil, err
	}
	return pairEvalSet(ctx, decisions, truths, instructions)
}

// pairEvalSet matches each decision with its truth entry, one to one, and
// builds the questions in the order of the decisions.
func pairEvalSet(ctx context.Context, decisions []evalDecision, truths []evalTruth, instructions string) ([]evalItem, error) {
	byID := make(map[string]evalTruth, len(truths))
	for _, tr := range truths {
		byID[tr.ID] = tr
	}
	for _, d := range decisions {
		if _, ok := byID[d.ID]; !ok {
			return nil, evalFieldError(ctx, entryField("truth", d.ID, "id"), "decision %q has no truth entry", d.ID)
		}
	}
	if len(truths) != len(decisions) {
		seen := make(map[string]bool, len(decisions))
		for _, d := range decisions {
			seen[d.ID] = true
		}
		for _, tr := range truths {
			if !seen[tr.ID] {
				return nil, evalFieldError(ctx, entryField("decisions", tr.ID, "id"), "truth entry %q has no decision", tr.ID)
			}
		}
	}

	items := make([]evalItem, 0, len(decisions))
	for _, d := range decisions {
		item, err := buildEvalItem(ctx, d, byID[d.ID], instructions)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func buildEvalItem(ctx context.Context, d evalDecision, tr evalTruth, instructions string) (evalItem, error) {
	field := func(sub string) string { return entryField("decisions", d.ID, sub) }
	entries, err := optionEntries(ctx, d, field)
	if err != nil {
		return evalItem{}, err
	}

	class, err := eval.ParseClass(tr.Class)
	if err != nil {
		return evalItem{}, evalFieldError(ctx, entryField("truth", d.ID, "class"), "%q is neither dominant nor contested", tr.Class)
	}
	correct, err := correctOption(ctx, entryField("truth", d.ID, "correct_option"), class, tr.CorrectOption, entries)
	if err != nil {
		return evalItem{}, err
	}

	question, err := decisionQuestion(ctx, d, entries, instructions, field)
	if err != nil {
		return evalItem{}, err
	}
	return evalItem{id: d.ID, class: class, correct: correct, question: question}, nil
}

// optionEntries reads the name and description of each option. field names
// the part of the decision at fault in an error.
func optionEntries(ctx context.Context, d evalDecision, field func(string) string) ([]decision.SpecEntry, error) {
	entries := make([]decision.SpecEntry, len(d.Options))
	for i, raw := range d.Options {
		var opt struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal(raw, &opt); err != nil || opt.Name == "" {
			return nil, evalFieldError(ctx, field(fmt.Sprintf("options[%d].name", i)), "an option needs a name")
		}
		entries[i] = decision.SpecEntry{Name: opt.Name, Description: opt.Description}
	}
	return entries, nil
}

// decisionQuestion builds the choice question eval asks for d, which also
// applies every check ask applies to a spec.
func decisionQuestion(ctx context.Context, d evalDecision, entries []decision.SpecEntry, instructions string, field func(string) string) (decision.Question[string], error) {
	spec := decision.Spec{
		State: map[string]any{
			"title":       d.Title,
			"context":     d.Context,
			"constraints": d.Constraints,
			"options":     d.Options,
		},
		Instructions: instructions,
		Options:      entries,
	}
	question, err := spec.Question()
	if err != nil {
		sub := "question"
		var fe *decision.FieldError
		if errors.As(err, &fe) {
			sub = fe.Field
		}
		return decision.Question[string]{}, evalFieldError(ctx, field(sub), "%v", err)
	}
	return question, nil
}

// correctOption checks a truth entry's correct option against its class and
// the decision's options. field names the correct option in an error.
func correctOption(ctx context.Context, field string, class eval.Class, given *string, entries []decision.SpecEntry) (string, error) {
	if class == eval.Contested {
		if given != nil {
			return "", evalFieldError(ctx, field, "a contested decision has no correct option, got %q", *given)
		}
		return "", nil
	}
	if given == nil || *given == "" {
		return "", evalFieldError(ctx, field, "a dominant decision needs a correct option")
	}
	for _, e := range entries {
		if e.Name == *given {
			return *given, nil
		}
	}
	return "", evalFieldError(ctx, field, "%q is not one of the decision's options", *given)
}

// readEntries decodes a JSON array of entries. Each entry is decoded with
// unknown fields disallowed, and its id must be present and unique.
func readEntries[T evalDecision | evalTruth](ctx context.Context, name, path string) ([]T, error) {
	flag := "--" + name
	if path == "" {
		return nil, evalFieldError(ctx, flag, "is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, evalFieldError(ctx, flag, "%v", err)
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, maxEvalFileBytes+1))
	if err != nil {
		return nil, evalFieldError(ctx, flag, "%v", err)
	}
	if len(data) > maxEvalFileBytes {
		return nil, evalFieldError(ctx, flag, "is larger than %d bytes", maxEvalFileBytes)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, evalFieldError(ctx, flag, "%s is empty", path)
	}

	var raws []json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return nil, evalFieldError(ctx, flag, "%s is not a JSON array: %v", path, err)
	}
	if len(raws) == 0 {
		return nil, evalFieldError(ctx, flag, "%s has no entries", path)
	}

	out := make([]T, 0, len(raws))
	seen := make(map[string]bool, len(raws))
	for i, raw := range raws {
		var id struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(raw, &id)
		label := id.ID
		if label == "" {
			label = fmt.Sprintf("#%d", i)
		}

		var entry T
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&entry); err != nil {
			return nil, evalFieldError(ctx, fmt.Sprintf("%s[%s]", name, label), "%v", err)
		}
		switch {
		case id.ID == "":
			return nil, evalFieldError(ctx, entryField(name, label, "id"), "is empty")
		case seen[id.ID]:
			return nil, evalFieldError(ctx, entryField(name, label, "id"), "is repeated")
		}
		seen[id.ID] = true
		out = append(out, entry)
	}
	return out, nil
}

func entryField(file, id, field string) string {
	return fmt.Sprintf("%s[%s].%s", file, id, field)
}

func evalFieldError(ctx context.Context, field, format string, args ...any) error {
	return newValidationError(ctx, field+": "+fmt.Sprintf(format, args...), field)
}

// loadEvalLog builds the eval set from a decision log written by record: each
// decision that has an outcome, with its first decision line and its last
// outcome line, in the order the decisions were recorded. It sends nothing.
func loadEvalLog(ctx context.Context, path, instructions string) ([]evalItem, error) {
	const flag = "--log"
	if _, err := os.Stat(path); err != nil {
		return nil, evalFieldError(ctx, flag, "%v", err)
	}
	lines, err := readLogLines(path)
	if err != nil {
		return nil, evalFieldError(ctx, flag, "%s: %v", path, err)
	}

	var order []string
	decisionLines := map[string]json.RawMessage{}
	truthLines := map[string]json.RawMessage{}
	for _, l := range lines {
		switch l.Kind {
		case kindDecision:
			if _, seen := decisionLines[l.ID]; !seen {
				decisionLines[l.ID] = l.Decision
				order = append(order, l.ID)
			}
		case kindOutcome:
			truthLines[l.ID] = l.Truth
		}
	}

	var decisions []evalDecision
	var truths []evalTruth
	for _, id := range order {
		truthRaw, ok := truthLines[id]
		if !ok {
			continue
		}
		var d evalDecision
		if err := decodeStrict(string(decisionLines[id]), &d); err != nil {
			return nil, evalFieldError(ctx, entryField("log", id, "decision"), "%v", err)
		}
		var tr evalTruth
		if err := decodeStrict(string(truthRaw), &tr); err != nil {
			return nil, evalFieldError(ctx, entryField("log", id, "truth"), "%v", err)
		}
		decisions = append(decisions, d)
		truths = append(truths, tr)
	}
	if len(decisions) == 0 {
		return nil, evalFieldError(ctx, flag, "%s has no labelled decision; record an outcome first", path)
	}
	return pairEvalSet(ctx, decisions, truths, instructions)
}
