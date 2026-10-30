package main

import (
	"os"

	"github.com/akimon658/traq-cli/internal/ui/cli"
)

func main() {
	os.Exit(cli.Execute())
}
