package main

import (
	"os"

	"github.com/spf13/cobra"
)

// jsonOutput is the global --json flag.
var jsonFlag bool

func main() {
	rootCommand := cobra.Command{
		Use: "vai",
	}

	rootCommand.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Output in JSON format")

	rootCommand.AddCommand(buildCommand())
	rootCommand.AddCommand(genCommand())
	rootCommand.AddCommand(initCommand())
	rootCommand.AddCommand(promptCommand())
	rootCommand.AddCommand(treeCommand())
	if err := rootCommand.Execute(); err != nil {
		os.Exit(1)
	}
}
