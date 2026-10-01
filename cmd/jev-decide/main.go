// Command jev-decide asks Jev a choice or score question and prints a typed
// outcome. See the ask and score commands.
package main

import (
	"context"
	"os"

	"github.com/rshade/jev-decide/internal/cli"
)

func main() {
	os.Exit(cli.Run(context.Background(), os.Args[1:], cli.Env{}))
}
