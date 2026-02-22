package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vailang/vai/internal/compiler"
	"github.com/vailang/vai/internal/config"
)

func buildCommand() *cobra.Command {
	var evalExpr string

	cmd := &cobra.Command{
		Use:   "build [file.vai]",
		Short: "Build vai files",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			comp := compiler.New()

			var prog compiler.Program
			var errs []error

			if len(args) == 1 {
				// Standalone single-file mode.
				prog, errs = comp.Parse(args[0])
			} else {
				// Package mode: find vai.toml, load files.
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				cfgPath, err := config.FindConfig(cwd)
				if err != nil {
					return fmt.Errorf("no vai.toml found (run 'vai init <name>' to create one)")
				}
				cfg, err := config.LoadConfig(cfgPath)
				if err != nil {
					return err
				}
				pkg := &config.Package{
					Name:       cfg.Lib.Name,
					ConfigPath: cfgPath,
					RootDir:    filepath.Dir(cfgPath),
					SrcDir:     filepath.Join(filepath.Dir(cfgPath), cfg.Lib.Prompts),
					Config:     cfg,
				}
				files, err := config.LoadPackageFiles(pkg)
				if err != nil {
					return err
				}
				if len(files) == 0 {
					return fmt.Errorf("no .vai or .plan files found in package %q", pkg.Name)
				}
				prog, errs = comp.ParseSources(files)
			}

			if len(errs) > 0 {
				for _, e := range errs {
					fmt.Fprintf(os.Stderr, "error: %v\n", e)
				}
				return fmt.Errorf("compilation failed with %d error(s)", len(errs))
			}

			if evalExpr != "" {
				output, err := prog.Eval(evalExpr)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", err)
					return fmt.Errorf("eval failed")
				}
				fmt.Print(output)
				return nil
			}

			fmt.Print(prog.Render())
			return nil
		},
	}

	cmd.Flags().StringVar(&evalExpr, "eval", "", "Evaluate a vai expression against the loaded files")

	return cmd
}
