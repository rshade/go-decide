package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"github.com/rshade/ax-go/contract"
	"github.com/spf13/cobra"

	"github.com/rshade/go-decide/decision"
)

const specFlagUsage = "decision spec as a JSON file, or - for standard input"

type inputFlags struct {
	// serving is set while mcp-server runs. A tool caller must not make the
	// server read a file or its standard input, so --spec is refused then.
	serving      *atomic.Bool
	spec         string
	state        string
	instructions string
	floor        float64
	confident    float64
	backend      backendFlag
}

func (f *inputFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.spec, "spec", "", specFlagUsage)
	cmd.Flags().StringVar(&f.state, "state", "", "the content to decide on, as a string")
	cmd.Flags().StringVar(&f.instructions, "instructions", "", "what to decide")
	f.backend.register(cmd)
	f.registerThresholds(cmd)
}

func (f *inputFlags) registerThresholds(cmd *cobra.Command) {
	cmd.Flags().Float64Var(&f.floor, "floor", 0, "confidence below which the outcome is escalate (default 0.5, a placeholder until #6)")
	cmd.Flags().Float64Var(&f.confident, "confident", 0, "confidence at or above which the outcome is decided (default 0.9, a placeholder until #6)")
}

// loadSpec reads the document named by --spec and merges the flag values into it.
// entries are the repeated --option or --level values.
func (f *inputFlags) loadSpec(cmd *cobra.Command, entries []decision.SpecEntry, forLevels bool) (decision.Spec, error) {
	var doc decision.Spec
	if f.spec != "" && f.serving != nil && f.serving.Load() {
		return decision.Spec{}, newValidationError(cmd.Context(),
			"--spec is not available over MCP: pass state, instructions and the options or levels inline", "spec")
	}
	if f.spec != "" {
		var err error
		if doc, err = readSpec(cmd, f.spec); err != nil {
			return decision.Spec{}, validationError(cmd.Context(), err)
		}
	}
	flags := decision.Spec{Instructions: f.instructions}
	if cmd.Flags().Changed("state") {
		flags.State = f.state
	}
	if forLevels {
		flags.Levels = entries
	} else {
		flags.Options = entries
	}
	merged, err := doc.MergeFlags(flags)
	if err != nil {
		return decision.Spec{}, validationError(cmd.Context(), err)
	}
	return merged, nil
}

func readSpec(cmd *cobra.Command, source string) (decision.Spec, error) {
	if source == "-" {
		return decision.ParseSpec(cmd.InOrStdin())
	}
	file, err := os.Open(source)
	if err != nil {
		return decision.Spec{}, fmt.Errorf("--spec: %w", err)
	}
	defer func() { _ = file.Close() }()
	return decision.ParseSpec(file)
}

// thresholds returns the backend defaults with any --floor or --confident applied.
func (f *inputFlags) thresholds(cmd *cobra.Command, b backend) (decision.Thresholds, error) {
	th := b.thresholds
	floor, confident := th.Floor().Float64(), th.Confident().Float64()
	if cmd.Flags().Changed("floor") {
		floor = f.floor
	}
	if cmd.Flags().Changed("confident") {
		confident = f.confident
	}
	fp, err := decision.NewProbability(floor)
	if err != nil {
		return decision.Thresholds{}, validationError(cmd.Context(), fmt.Errorf("--floor: %w", err))
	}
	cp, err := decision.NewProbability(confident)
	if err != nil {
		return decision.Thresholds{}, validationError(cmd.Context(), fmt.Errorf("--confident: %w", err))
	}
	th, err = decision.NewThresholds(fp, cp)
	if err != nil {
		return decision.Thresholds{}, validationError(cmd.Context(), err)
	}
	return th, nil
}

// parseEntries turns repeated name=description flag values into entries.
func parseEntries(ctx context.Context, flag string, values []string) ([]decision.SpecEntry, error) {
	entries := make([]decision.SpecEntry, len(values))
	for i, v := range values {
		name, description, _ := strings.Cut(v, "=")
		if name == "" {
			return nil, validationError(ctx, fmt.Errorf("--%s %q: expected name=description with a name", flag, v))
		}
		entries[i] = decision.SpecEntry{Name: name, Description: description}
	}
	return entries, nil
}

// validationError reports err as a failed validation: exit code 2, naming the
// offending field when err carries one.
func validationError(ctx context.Context, err error) error {
	var field *decision.FieldError
	if errors.As(err, &field) {
		return newValidationError(ctx, err.Error(), field.Field)
	}
	return newValidationError(ctx, err.Error(), "")
}

// newValidationError builds the exit-code-2 envelope, with field in its
// context when it is not empty.
func newValidationError(ctx context.Context, msg, field string) error {
	opts := []contract.ErrorOption{contract.WithErrorExitCode(contract.ExitValidation)}
	if field != "" {
		opts = append(opts, contract.WithErrorContext(map[string]any{"field": field}))
	}
	return contract.NewError(ctx, "validation_error", msg, opts...)
}

// failure turns a failed call into the error a command returns: a rejected
// input is a validation failure, and anything else, already classified by
// jevclient, passes through.
func failure(ctx context.Context, err error) error {
	for _, invalid := range []error{
		decision.ErrInvalidSpec, decision.ErrInvalidQuestion, decision.ErrInvalidOptions,
		decision.ErrInvalidLevels, decision.ErrInvalidThresholds,
	} {
		if errors.Is(err, invalid) {
			return validationError(ctx, err)
		}
	}
	return err
}
