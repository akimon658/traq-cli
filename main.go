package main

import (
	"os"

	"github.com/akimon658/traq-cli/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
