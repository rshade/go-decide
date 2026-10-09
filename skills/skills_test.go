package skills_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/rshade/go-decide/skills"
)

func TestDecideFilesMatchTheRepository(t *testing.T) {
	for _, name := range []string{"SKILL.md", "references/debate-prompts.md"} {
		t.Run(name, func(t *testing.T) {
			embedded, err := fs.ReadFile(skills.Decide(), name)
			if err != nil {
				t.Fatalf("reading embedded %s: %v", name, err)
			}
			onDisk, err := os.ReadFile(filepath.Join("decide", filepath.FromSlash(name)))
			if err != nil {
				t.Fatalf("reading decide/%s: %v", name, err)
			}
			if string(embedded) != string(onDisk) {
				t.Fatalf("embedded %s differs from decide/%s", name, name)
			}
		})
	}
}

func TestDecideServesOnlyMarkdown(t *testing.T) {
	err := fs.WalkDir(skills.Decide(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) != ".md" {
			t.Errorf("embedded %s is not Markdown", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
