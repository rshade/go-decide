package cli

import (
	"fmt"

	ax "github.com/rshade/ax-go"
	"github.com/spf13/cobra"
)

// outputSchema is the __schema document. schema_version is this module's
// integer, not ax-go's envelope version string.
type outputSchema struct {
	SchemaVersion int                `json:"schema_version"`
	Tool          string             `json:"tool"`
	Version       string             `json:"version"`
	ModeDetection string             `json:"mode_detection"`
	Command       ax.CommandSchema   `json:"command"`
	ErrorEnvelope ax.ErrorSchemaInfo `json:"error_envelope"`
}

func newSchemaCommand(root *cobra.Command) *cobra.Command {
	var as string
	cmd := &cobra.Command{
		Use:   "__schema",
		Short: "Emit the AX machine-discoverability schema",
		Example: fmt.Sprintf("  %s __schema\n  %s __schema --as=mcp",
			root.Name(), root.Name()),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			switch as {
			case "", "ax":
				doc := ax.BuildSchema(root, ax.WithSchemaVersion(root.Version))
				return ax.WriteJSON(cmd.OutOrStdout(), outputSchema{
					SchemaVersion: SchemaVersion,
					Tool:          doc.Tool,
					Version:       doc.Version,
					ModeDetection: doc.ModeDetection,
					Command:       doc.Command,
					ErrorEnvelope: doc.ErrorEnvelope,
				})
			case "mcp":
				return ax.WriteJSON(cmd.OutOrStdout(), ax.BuildMCPSchema(root))
			default:
				return ax.NewError(cmd.Context(), "validation_error",
					fmt.Sprintf("unknown schema format %q", as),
					ax.WithErrorExitCode(ax.ExitValidation))
			}
		},
	}
	cmd.Flags().StringVar(&as, "as", "ax", "schema format: ax or mcp")
	return cmd
}
