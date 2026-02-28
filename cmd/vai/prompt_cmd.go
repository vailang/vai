package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"
	"github.com/vailang/vai/internal/config"
	"github.com/vailang/vai/internal/prompter"
)

func promptCommand() *cobra.Command {
	var (
		yesFlag     bool
		verboseFlag bool
	)

	cmd := &cobra.Command{
		Use:   "prompt <message>",
		Short: "Use natural language to create vai plans and generate code",
		Long:  "Takes a natural language prompt, uses an LLM to create/modify .vai/.plan files and target stubs, then runs the generation pipeline.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			message := args[0]

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

			// Validate planner config.
			if cfg.Planner.Provider == "" && cfg.Planner.BaseURL == "" {
				return fmt.Errorf("no [planner] configured in vai.toml")
			}

			// Run prompter.
			p, err := prompter.New(cfg, baseDir)
			if err != nil {
				return err
			}

			var (
				wg      sync.WaitGroup
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

			result, runErr := p.Run(cmd.Context(), message)
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
				fmt.Println("No changes proposed.")
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

			fmt.Fprintln(os.Stderr, "\nRunning generation...")

			// Run the full generation pipeline.
			r, _, err := setupGenRunner()
			if err != nil {
				return fmt.Errorf("setting up generation: %w", err)
			}
			events, errc := r.Run(cmd.Context())
			return consumeEvents(events, errc)
		},
	}

	cmd.Flags().BoolVarP(&yesFlag, "yes", "y", false, "Accept all changes without confirmation")
	cmd.Flags().BoolVarP(&verboseFlag, "verbose", "v", false, "Show detailed tool calls, results, and LLM responses")
	return cmd
}
