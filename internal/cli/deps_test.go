package cli

import (
	"os/exec"
	"strings"
	"testing"
)

func TestLibraryPackagesStayFreeOfTheRuntimeDependencies(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go list; skipped with -short")
	}
	out, err := exec.Command("go", "list", "-deps", "../../decision", "../../jevclient", "../../eval").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps: %v\n%s", err, out)
	}

	for _, pkg := range strings.Fields(string(out)) {
		for _, heavy := range []string{"go.opentelemetry.io/otel/sdk", "google.golang.org/grpc", "github.com/spf13/cobra", "github.com/rshade/ax-go/telemetry"} {
			if strings.HasPrefix(pkg, heavy) {
				t.Errorf("the library packages depend on %s, which only the CLI should link", pkg)
			}
		}
	}
	if !strings.Contains(string(out), "github.com/rshade/ax-go/contract") {
		t.Error("expected the library to import ax-go/contract, so this check proves nothing")
	}
}
