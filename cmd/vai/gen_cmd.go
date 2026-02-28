package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"
	"github.com/vailang/vai/internal/compiler"
	"github.com/vailang/vai/internal/config"
	"github.com/vailang/vai/internal/locker"
	"github.com/vailang/vai/internal/prompter"
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
	var (
		yesFlag     bool
		verboseFlag bool
	)

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Review and reshape plan specs via LLM before the skeleton step",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Find project root.
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
			baseDir := filepath.Dir(cfgPath)

			if cfg.Planner.Provider == "" && cfg.Planner.BaseURL == "" {
				return fmt.Errorf("no [planner] configured in vai.toml")
			}

			// Load the plan reviewer system prompt from std library.
			comp := compiler.New()
			comp.SetBaseDir(baseDir)
			var systemPrompt string
			prog, errs := comp.ParseSources(map[string]string{})
			if len(errs) == 0 && prog.HasPrompt("std.vai_plan_reviewer") {
				result, err := prog.Eval("inject std.vai_plan_reviewer")
				if err == nil && result != "" {
					systemPrompt = result
				}
			}

			// Create prompter with reviewer prompt.
			p, err := prompter.New(cfg, baseDir)
			if err != nil {
				return err
			}
			if systemPrompt != "" {
				p.SetSystemPrompt(systemPrompt)
			}

			var (
				wg           sync.WaitGroup
				promptEvents chan prompter.Event
			)
			if !jsonFlag {
				promptEvents = make(chan prompter.Event, 32)
				p.SetEvents(promptEvents)
				display := prompter.NewDisplay(verboseFlag)
				wg.Add(1)
				go func() {
					defer wg.Done()
					display.Consume(promptEvents)
				}()
			}

			// Build user prompt based on --name flag.
			userPrompt := "Review and reshape all plan specs. Approve specs that are already clear, update those that need improvement."
			if nameFlag != "" {
				userPrompt = fmt.Sprintf("Review and reshape the spec for plan %q. Approve it if already clear, update it if it needs improvement.", nameFlag)
			}

			result, runErr := p.Run(cmd.Context(), userPrompt)
			if promptEvents != nil {
				close(promptEvents)
				wg.Wait()
			}
			if runErr != nil {
				return runErr
			}

			if len(result.Changes) == 0 {
				if jsonFlag {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(map[string]any{
						"changes": []any{},
						"summary": result.Summary,
					})
				}
				fmt.Println(result.Summary)
				return nil
			}

			// Display changes.
			if jsonFlag {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"changes":    result.Changes,
					"summary":    result.Summary,
					"tokens_in":  result.TokensIn,
					"tokens_out": result.TokensOut,
				})
			}

			prompter.DisplayChanges(result.Changes, os.Stdout)

			// Confirm unless -y.
			if !yesFlag {
				if !prompter.Confirm(os.Stdin, os.Stdout) {
					fmt.Println("Aborted.")
					return nil
				}
			}

			// Flush changes to disk.
			if err := result.Flush(baseDir); err != nil {
				return fmt.Errorf("writing changes: %w", err)
			}

			fmt.Println("Specs updated.")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&yesFlag, "yes", "y", false, "Accept all changes without confirmation")
	cmd.Flags().BoolVarP(&verboseFlag, "verbose", "v", false, "Show detailed tool calls, results, and LLM responses")
	return cmd
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
