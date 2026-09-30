package jevclient

import (
	"os"
	"strings"
	"testing"
)

func TestErrorDocsListEveryCode(t *testing.T) {
	doc, err := os.ReadFile("../docs/jev-errors.md")
	if err != nil {
		t.Fatalf("reading error docs: %v", err)
	}
	for _, code := range []string{
		CodeUnauthorized,
		CodeInvalidRequest,
		CodeRateLimited,
		CodeOverloaded,
		CodeStatus,
		CodeTimeout,
		CodeConnection,
		CodeInvalidResponse,
		CodeInternal,
	} {
		row := "| `" + code + "` |"
		if !strings.Contains(string(doc), row) {
			t.Errorf("docs/jev-errors.md has no table row for %s", code)
		}
		if !strings.Contains(string(doc), `"error_code": "`+code+`"`) {
			t.Errorf("docs/jev-errors.md has no JSON example for %s", code)
		}
	}
}
