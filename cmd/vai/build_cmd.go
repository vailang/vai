package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vailang/vai/internal/compiler"
)

func buildCommand() *cobra.Command {
	var evalExpr string

	cmd := &cobra.Command{
		Use:   "build [file.vai]",
		Short: "Build vai files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			comp := compiler.New()

			if evalExpr != "" {
				output, err := comp.Eval(args[0], evalExpr)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", err)
					return fmt.Errorf("eval failed")
				}
				fmt.Print(output)
				return nil
			}

			prog, errs := comp.Parse(args[0])
			if len(errs) > 0 {
				for _, e := range errs {
					fmt.Fprintf(os.Stderr, "error: %v\n", e)
				}
				return fmt.Errorf("compilation failed with %d error(s)", len(errs))
			}
			fmt.Print(prog.Render())
			return nil
		},
	}

	cmd.Flags().StringVar(&evalExpr, "eval", "", "Evaluate a vai expression against the loaded files")

	return cmd
}
