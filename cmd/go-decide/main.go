// Command go-decide asks a System One model, Jev by default, a choice or score
// question and prints a typed outcome. See the ask and score commands.
package main

import (
	"context"
	"os"

	"github.com/rshade/go-decide/internal/cli"
)

// version is set at release time by GoReleaser through -ldflags. Left empty,
// the version comes from the module build info.
var version string

func main() {
	os.Exit(cli.Run(context.Background(), os.Args[1:], cli.Env{Version: version}))
}
