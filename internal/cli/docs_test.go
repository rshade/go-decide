package cli

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/rshade/ax-go/contract"
)

func TestDocsListEveryExitCode(t *testing.T) {
	doc, err := os.ReadFile("../../docs/jev-decide-cli.md")
	if err != nil {
		t.Fatalf("reading the CLI docs: %v", err)
	}

	for _, code := range []int{
		contract.ExitSuccess, contract.ExitInternal, contract.ExitValidation, contract.ExitNetwork, contract.ExitAuth,
		ExitUncertain, ExitEscalate,
	} {
		if row := fmt.Sprintf("| %d |", code); !strings.Contains(string(doc), row) {
			t.Errorf("docs/jev-decide-cli.md has no exit code table row %q", row)
		}
	}
}
