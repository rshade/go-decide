package clefclient

import (
	"os"
	"strings"
	"testing"
)

func TestErrorDocsListEveryCode(t *testing.T) {
	doc, err := os.ReadFile("../docs/clef-errors.md")
	if err != nil {
		t.Fatalf("reading error docs: %v", err)
	}
	for _, code := range []string{
		CodeUnauthorized,
		CodeInvalidRequest,
		CodeRateLimited,
		CodeStatus,
		CodeTimeout,
		CodeConnection,
		CodeInvalidResponse,
		CodeInternal,
	} {
		if !strings.Contains(string(doc), "| `"+code+"` |") {
			t.Errorf("docs/clef-errors.md has no table row for %s", code)
		}
		if !strings.Contains(string(doc), `"error_code": "`+code+`"`) {
			t.Errorf("docs/clef-errors.md has no JSON example for %s", code)
		}
	}
}
