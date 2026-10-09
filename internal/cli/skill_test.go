package cli

import "testing"

func TestReadFrontmatterNeedsNameAndDescription(t *testing.T) {
	for name, doc := range map[string]string{
		"no frontmatter":    "# decide\n",
		"unclosed":          "---\nname: decide\ndescription: x\n",
		"no description":    "---\nname: decide\n---\n# decide\n",
		"no name":           "---\ndescription: x\n---\n# decide\n",
		"not yaml":          "---\nname: [decide\n---\n",
		"blank description": "---\nname: decide\ndescription: >\n  \n---\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := readFrontmatter([]byte(doc)); err == nil {
				t.Errorf("readFrontmatter(%q) succeeded, want an error", doc)
			}
		})
	}
}

func TestReadFrontmatterFoldsTheDescription(t *testing.T) {
	front, err := readFrontmatter([]byte("---\nname: decide\ndescription: >\n  Pick one.\n  Use when choosing.\n---\n# decide\n"))
	if err != nil {
		t.Fatal(err)
	}
	if front.Name != "decide" || front.Description != "Pick one. Use when choosing." {
		t.Errorf("got %+v, want name decide and a one-line description", front)
	}
}
