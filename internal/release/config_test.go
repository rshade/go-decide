// Package release guards the release configuration, so a setting that breaks
// the first release fails a test instead of a tag.
package release

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"
)

var semver = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

type releasePleasePackage struct {
	ReleaseType               string `json:"release-type"`
	BumpMinorPreMajor         bool   `json:"bump-minor-pre-major"`
	BumpPatchForMinorPreMajor bool   `json:"bump-patch-for-minor-pre-major"`
	InitialVersion            string `json:"initial-version"`
	IncludeComponentInTag     *bool  `json:"include-component-in-tag"`
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
}

func TestReleasePleaseConfig(t *testing.T) {
	var cfg struct {
		Packages map[string]releasePleasePackage `json:"packages"`
	}
	readJSON(t, "../../release-please-config.json", &cfg)
	pkg, ok := cfg.Packages["."]
	if !ok {
		t.Fatal(`release-please-config.json has no "." package`)
	}

	if pkg.ReleaseType != "go" {
		t.Errorf("release-type = %q, want go", pkg.ReleaseType)
	}
	if pkg.IncludeComponentInTag == nil || *pkg.IncludeComponentInTag {
		t.Error("include-component-in-tag must be false: a component-prefixed tag is not semver, and GoReleaser fails on it")
	}
	if pkg.InitialVersion != "0.1.0" {
		t.Errorf("initial-version = %q, want 0.1.0, or the first release PR can propose 1.0.0", pkg.InitialVersion)
	}
	if !pkg.BumpMinorPreMajor || !pkg.BumpPatchForMinorPreMajor {
		t.Error("before 1.0 a breaking change bumps the minor and a feat bumps the patch; minors are bumped by hand")
	}
}

func TestReleasePleaseManifestFormat(t *testing.T) {
	var manifest map[string]string
	readJSON(t, "../../.release-please-manifest.json", &manifest)
	if len(manifest) != 1 {
		t.Errorf("manifest has %d entries, want only the %q package", len(manifest), ".")
	}
	if !semver.MatchString(manifest["."]) {
		t.Errorf(`manifest "." = %q, want X.Y.Z with no v prefix; the release PR rewrites it on every release`, manifest["."])
	}
}
