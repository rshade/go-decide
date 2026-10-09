package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/rshade/ax-go/schema"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"

	"github.com/rshade/go-decide/skills"
)

// skillBase is the URI the decide skill's files are served under. Each file's
// URI mirrors its path in skills/decide/, so the relative references inside
// SKILL.md resolve against the SKILL.md URI.
const skillBase = "go-decide://skills/decide/"

const skillURI = skillBase + "SKILL.md"

const decidePrompt = "Run the decide skill on this decision: {{decision}}\n\n" +
	"Read the MCP resource " + skillURI + " from the go-decide server and follow it. " +
	"Its references/ paths resolve against that URI."

// skillFrontmatter is the part of SKILL.md's YAML frontmatter the server
// reuses, so what an agent is told about the skill is what the skill says
// about itself.
type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// declareSkill declares every file of the embedded decide skill as a static
// resource on root, and the decide prompt that points at it. It returns the
// server instructions, built from the skill's own description.
func declareSkill(root *cobra.Command) (string, error) {
	files := skills.Decide()
	doc, err := fs.ReadFile(files, "SKILL.md")
	if err != nil {
		return "", fmt.Errorf("reading the decide skill: %w", err)
	}
	front, err := readFrontmatter(doc)
	if err != nil {
		return "", fmt.Errorf("reading the decide skill's frontmatter: %w", err)
	}
	err = fs.WalkDir(files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		content, err := fs.ReadFile(files, path)
		if err != nil {
			return err
		}
		description := "Referenced by the " + front.Name + " skill's SKILL.md as " + path + "."
		if path == "SKILL.md" {
			description = front.Description
		}
		return schema.DeclareResource(root, schema.Resource{
			URI:         skillBase + path,
			Name:        front.Name + "/" + path,
			Title:       front.Name + " skill: " + path,
			Description: description,
			MIMEType:    "text/markdown",
			Content:     string(content),
		})
	})
	if err != nil {
		return "", fmt.Errorf("declaring the decide skill: %w", err)
	}
	err = schema.DeclarePrompt(root, schema.Prompt{
		Name:        front.Name,
		Title:       "Run the " + front.Name + " skill",
		Description: front.Description,
		Arguments: []schema.PromptArgument{{
			Name:        "decision",
			Description: "The decision and the options being weighed.",
			Required:    true,
		}},
		Template: decidePrompt,
	})
	if err != nil {
		return "", fmt.Errorf("declaring the decide prompt: %w", err)
	}
	return serverInstructions(front), nil
}

// serverInstructions sits in every connected agent's context, so it carries
// a pointer and the skill's own description, never the skill itself. The
// pointer comes first and names the shapes a choice arrives in: in Claude Code
// trials, a pointer after the description, without those shapes, was skipped
// for "should we X or Y" and "thoughts on switching" questions. The last
// sentence is about this server's tools, which the skill does not describe.
func serverInstructions(front skillFrontmatter) string {
	return "When the user is weighing options, including \"should we X or Y\", \"is it worth adopting X\", \"help me pick\" and \"thoughts on switching to X\", " +
		"do not answer from your own judgment first: read the MCP resource " + skillURI + " and follow it. " +
		"It settles a clear choice with one pre-screen call and debates only an unclear one." +
		"\n\nThe " + front.Name + " skill: " + front.Description + "\n\n" +
		"Every go-decide-ask and go-decide-score call is a paid request, and a decided outcome is not approval."
}

func readFrontmatter(doc []byte) (skillFrontmatter, error) {
	rest, ok := bytes.CutPrefix(doc, []byte("---\n"))
	if !ok {
		return skillFrontmatter{}, errors.New("no frontmatter: SKILL.md must start with ---")
	}
	block, _, ok := bytes.Cut(rest, []byte("\n---\n"))
	if !ok {
		return skillFrontmatter{}, errors.New("frontmatter is not closed with ---")
	}
	var front skillFrontmatter
	if err := yaml.Unmarshal(block, &front); err != nil {
		return skillFrontmatter{}, err
	}
	front.Name = strings.TrimSpace(front.Name)
	front.Description = strings.Join(strings.Fields(front.Description), " ")
	if front.Name == "" || front.Description == "" {
		return skillFrontmatter{}, errors.New("frontmatter needs a name and a description")
	}
	return front, nil
}
