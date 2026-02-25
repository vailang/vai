package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vailang/vai/internal/compiler"
	"github.com/vailang/vai/internal/config"
	"github.com/vailang/vai/internal/locker"
	"github.com/vailang/vai/internal/runner"
	"github.com/vailang/vai/internal/ui"
)

var (
	nameFlag  string
	forceFlag bool
)

// setupGenRunner handles the shared setup for all gen sub-commands:
// find config, compile program, create runner.
// If nameFlag is set, validates that the plan exists and sets the filter.
func setupGenRunner() (*runner.Runner, compiler.Program, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, nil, err
	}
	cfgPath, err := config.FindConfig(cwd)
	if err != nil {
		return nil, nil, fmt.Errorf("no vai.toml found (run 'vai init <name>' to create one)")
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		return nil, nil, err
	}
	baseDir := filepath.Dir(cfgPath)

	comp := compiler.New()
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
		return nil, nil, err
	}
	if len(files) == 0 {
		return nil, nil, fmt.Errorf("no .vai or .plan files found in package %q", pkg.Name)
	}

	prog, errs := comp.ParseSources(files)
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "error: %v\n", e)
		}
		return nil, nil, fmt.Errorf("compilation failed with %d error(s)", len(errs))
	}

	// Print warnings to stderr (non-fatal).
	for _, w := range prog.Warnings() {
		fmt.Fprintf(os.Stderr, "warning: %v\n", w)
	}

	if prog.Tasks() == 0 {
		return nil, nil, fmt.Errorf("no tasks found in program")
	}

	// Validate --name flag if provided.
	if nameFlag != "" && !prog.HasPlan(nameFlag) {
		return nil, nil, fmt.Errorf("plan %q not found", nameFlag)
	}

	// Create locker (nil when --force is set to skip lock checking).
	var lk runner.RequestLocker
	if !forceFlag {
		lk = locker.New(baseDir)
	}

	r, err := runner.New(prog, cfg, baseDir, lk)
	if err != nil {
		return nil, nil, err
	}

	if nameFlag != "" {
		r.SetPlanFilter(nameFlag)
	}

	return r, prog, nil
}

// consumeEvents creates a UI and consumes runner events.
func consumeEvents(events <-chan runner.Event, errc <-chan error) error {
	mode := ui.ModeTerminal
	if jsonFlag {
		mode = ui.ModeJSON
	}
	display := ui.New(mode)
	display.Consume(events)
	return <-errc
}

func genCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gen",
		Short: "Generate target code via the LLM pipeline",
		Long:  "Finds vai.toml in the current directory (or parents), loads all .vai/.plan files from the configured prompts directory, and runs the full generation pipeline.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, _, err := setupGenRunner()
			if err != nil {
				return err
			}
			events, errc := r.Run(cmd.Context())
			return consumeEvents(events, errc)
		},
	}

	cmd.PersistentFlags().StringVar(&nameFlag, "name", "", "Run only the specified plan (error if not found)")
	cmd.PersistentFlags().BoolVar(&forceFlag, "force", false, "Ignore lock file and re-execute all requests")

	cmd.AddCommand(genSkeletonCommand())
	cmd.AddCommand(genPlanCommand())
	cmd.AddCommand(genCodeCommand())

	return cmd
}

func genSkeletonCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "skeleton",
		Short: "Run architect + diff + flush (writes stubs to target files)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, _, err := setupGenRunner()
			if err != nil {
				return err
			}
			events, errc := r.RunSkeleton(cmd.Context())
			return consumeEvents(events, errc)
		},
	}
}

func genPlanCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "plan",
		Short: "Run architect + diff + save + flush (everything before executor)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, _, err := setupGenRunner()
			if err != nil {
				return err
			}
			events, errc := r.RunPlan(cmd.Context())
			return consumeEvents(events, errc)
		},
	}
}

func genCodeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "code",
		Short: "Run executor + debug (requires skeleton already saved in .vai file)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, _, err := setupGenRunner()
			if err != nil {
				return err
			}
			events, errc := r.RunCode(cmd.Context())
			return consumeEvents(events, errc)
		},
	}
}
