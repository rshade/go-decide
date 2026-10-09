// Package skills embeds the agent skills shipped in this repository, so the
// go-decide binary can serve them over MCP from the same files a skill
// installer reads. It exists for the binary and has no other API.
package skills

import (
	"embed"
	"io/fs"
)

//go:embed decide/SKILL.md decide/references/*.md
var files embed.FS

// Decide returns the decide skill's files, rooted at its directory:
// SKILL.md and references/*.md.
func Decide() fs.FS {
	sub, err := fs.Sub(files, "decide")
	if err != nil {
		panic(err)
	}
	return sub
}
