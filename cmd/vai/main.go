package main

import "github.com/spf13/cobra"

func main() {
	rootCommand := cobra.Command{
		Use: "vai",
	}

	rootCommand.AddCommand(buildCommand())
	if err := rootCommand.Execute(); err != nil {
		panic(err)
	}
}
