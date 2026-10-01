package decision

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// ErrInvalidSpec is wrapped by the error [ParseSpec] returns for a document that
// is not a decision spec.
var ErrInvalidSpec = errors.New("decision: invalid spec")

// maxSpecBytes bounds what [ParseSpec] reads. The API takes far less than this.
const maxSpecBytes = 4 << 20

// SpecEntry is one option or level of a [Spec]: a name and when it applies.
type SpecEntry struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Spec is a decision spec as read from input: the state, what to decide, and
// either options for a choice or levels for a score, in the order given.
// Unlike [Options], a Spec keeps the order and can hold a repeated name, so
// [Spec.Question] and [Spec.RateQuestion] are where duplicates are caught.
type Spec struct {
	State        any         `json:"state,omitempty"`
	Instructions string      `json:"instructions,omitempty"`
	Options      []SpecEntry `json:"options,omitempty"`
	Levels       []SpecEntry `json:"levels,omitempty"`
}

// ParseSpec reads one JSON decision spec from r. It rejects an empty document,
// malformed JSON, a field the format does not define and any content after the
// spec. Errors are *[FieldError] values wrapping [ErrInvalidSpec].
func ParseSpec(r io.Reader) (Spec, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxSpecBytes+1))
	if err != nil {
		return Spec{}, newFieldError(ErrInvalidSpec, "Spec", "cannot be read: %v", err)
	}
	if len(data) > maxSpecBytes {
		return Spec{}, newFieldError(ErrInvalidSpec, "Spec", "is larger than %d bytes", maxSpecBytes)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return Spec{}, newFieldError(ErrInvalidSpec, "Spec", "is empty")
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var spec Spec
	if err := dec.Decode(&spec); err != nil {
		return Spec{}, decodeError(err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Spec{}, newFieldError(ErrInvalidSpec, "Spec", "has content after the first JSON value")
	}
	return spec, nil
}

func decodeError(err error) error {
	var syntax *json.SyntaxError
	var typed *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntax):
		return newFieldError(ErrInvalidSpec, "Spec", "is not valid JSON (at byte %d): %v", syntax.Offset, err)
	case errors.Is(err, io.ErrUnexpectedEOF):
		return newFieldError(ErrInvalidSpec, "Spec", "is not valid JSON: it ends early")
	case errors.As(err, &typed):
		return newFieldError(ErrInvalidSpec, specFieldName(typed.Field), "must be a %s, not a %s", typed.Type, typed.Value)
	}
	if name, ok := strings.CutPrefix(err.Error(), "json: unknown field "); ok {
		return newFieldError(ErrInvalidSpec, strings.Trim(name, `"`), "is not a field of a decision spec")
	}
	return newFieldError(ErrInvalidSpec, "Spec", "is not a decision spec: %v", err)
}

func specFieldName(jsonPath string) string {
	if jsonPath == "" {
		return "Spec"
	}
	return jsonPath
}

// Question converts s to a choice question, checking it as [NewOptions] and
// [Question.Validate] do, and additionally rejecting a repeated option name. A
// spec that carries levels is not a choice. Errors are *[FieldError] values.
func (s Spec) Question() (Question[string], error) {
	if len(s.Levels) > 0 {
		if len(s.Options) == 0 {
			return Question[string]{}, newFieldError(ErrInvalidOptions, "Options",
				"a choice needs options, but the spec has levels (use score instead)")
		}
		return Question[string]{}, newFieldError(ErrInvalidSpec, "Levels", "a choice takes options, not levels")
	}
	if err := checkNames(s.Options, ErrInvalidOptions, "Options", "option"); err != nil {
		return Question[string]{}, err
	}
	descriptions := make(map[string]string, len(s.Options))
	for _, entry := range s.Options {
		descriptions[entry.Name] = entry.Description
	}
	options, err := NewOptions(descriptions)
	if err != nil {
		return Question[string]{}, err
	}
	q := Question[string]{State: s.State, Instructions: s.Instructions, Options: options}
	if err := q.Validate(); err != nil {
		return Question[string]{}, err
	}
	return q, nil
}

// RateQuestion converts s to a score question, checking it as [NewLevels] and
// [RateQuestion.Validate] do. A spec that carries options is not a rubric.
// Errors are *[FieldError] values.
func (s Spec) RateQuestion() (RateQuestion[string], error) {
	if len(s.Options) > 0 {
		if len(s.Levels) == 0 {
			return RateQuestion[string]{}, newFieldError(ErrInvalidLevels, "Levels",
				"a score needs levels, but the spec has options (use ask instead)")
		}
		return RateQuestion[string]{}, newFieldError(ErrInvalidSpec, "Options", "a score takes levels, not options")
	}
	levels := make([]Level[string], len(s.Levels))
	for i, entry := range s.Levels {
		levels[i] = Level[string](entry)
	}
	rubric, err := NewLevels(levels...)
	if err != nil {
		return RateQuestion[string]{}, err
	}
	q := RateQuestion[string]{State: s.State, Instructions: s.Instructions, Levels: rubric}
	if err := q.Validate(); err != nil {
		return RateQuestion[string]{}, err
	}
	return q, nil
}

func checkNames(entries []SpecEntry, kind error, field, noun string) error {
	seen := make(map[string]int, len(entries))
	for i, entry := range entries {
		if first, dup := seen[entry.Name]; dup && entry.Name != "" {
			return newFieldError(kind, field, "name %q is used by %ss %d and %d", entry.Name, noun, first+1, i+1)
		}
		seen[entry.Name] = i
	}
	return nil
}

// MergeFlags combines s, read from a document, with flags, the same fields given
// another way such as command-line flags. A field set in both is a conflict, never
// resolved silently, and is reported as a [*FieldError] naming it.
func (s Spec) MergeFlags(flags Spec) (Spec, error) {
	merged := s
	if flags.State != nil {
		if s.State != nil {
			return Spec{}, conflict("State")
		}
		merged.State = flags.State
	}
	if flags.Instructions != "" {
		if s.Instructions != "" {
			return Spec{}, conflict("Instructions")
		}
		merged.Instructions = flags.Instructions
	}
	if len(flags.Options) > 0 {
		if len(s.Options) > 0 {
			return Spec{}, conflict("Options")
		}
		merged.Options = flags.Options
	}
	if len(flags.Levels) > 0 {
		if len(s.Levels) > 0 {
			return Spec{}, conflict("Levels")
		}
		merged.Levels = flags.Levels
	}
	return merged, nil
}

func conflict(field string) error {
	return newFieldError(ErrInvalidSpec, field, "is given both in the spec document and by a flag")
}
