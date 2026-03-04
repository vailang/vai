package main

import (
	"encoding/json"
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
				// Single-file mode: try to find vai.toml for path resolution.
				absPath, _ := filepath.Abs(args[0])
				startDir := filepath.Dir(absPath)
				if cfgPath, cfgErr := config.FindConfig(startDir); cfgErr == nil {
					comp.SetBaseDir(filepath.Dir(cfgPath))
				}
				prog, errs = comp.Parse(args[0])
			} else {
				// Package mode: find vai.toml, load files.
				cfg, cfgPath, baseDir, err := loadProject()
				if err != nil {
					return err
				}
				comp.SetBaseDir(baseDir)
				pkg := &config.Package{
					Name:       cfg.Lib.Name,
					ConfigPath: cfgPath,
					RootDir:    baseDir,
					SrcDir:     filepath.Join(baseDir, cfg.Lib.Prompts),
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

			// Print warnings to stderr (non-fatal).
			for _, w := range prog.Warnings() {
				fmt.Fprintf(os.Stderr, "warning: %v\n", w)
			}

			if evalExpr != "" {
				output, err := prog.Eval(evalExpr)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", err)
					return fmt.Errorf("eval failed")
				}
				if jsonFlag {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(map[string]string{"output": output})
				}
				fmt.Print(output)
				return nil
			}

			output := prog.Render()
			if jsonFlag {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]string{"output": output})
			}
			fmt.Print(output)
			return nil
		},
	}

	cmd.Flags().StringVar(&evalExpr, "eval", "", "Evaluate a vai expression against the loaded files")

	return cmd
}
