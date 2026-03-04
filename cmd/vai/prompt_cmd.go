package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
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

			cfg, _, baseDir, err := loadProject()
			if err != nil {
				return err
			}

			// Validate planner config.
			if _, err := cfg.PlannerConfig(); err != nil {
				return fmt.Errorf("no LLM with role \"plan\" configured in vai.toml")
			}

			result, err := runPrompterFlow(cmd.Context(), prompterOpts{
				cfg:         cfg,
				baseDir:     baseDir,
				userPrompt:  message,
				verbose:     verboseFlag,
				autoYes:     yesFlag,
				noChangeMsg: "No changes proposed.",
			})
			if err != nil {
				return err
			}
			if len(result.Changes) == 0 {
				return nil
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
