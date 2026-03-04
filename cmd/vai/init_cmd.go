package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/vailang/vai/internal/config"
	"github.com/vailang/vai/internal/ui"
)

func initCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init <name>",
		Short: "Initialize a new vai package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			dest := filepath.Join(cwd, "vai.toml")
			if _, err := os.Stat(dest); err == nil {
				return fmt.Errorf("vai.toml already exists")
			}

			cfg := &config.Config{
				Lib: config.LibConfig{
					Name:    name,
					Prompts: "./prompts",
				},
			}

			// Interactive setup when stdin is a terminal.
			if ui.IsTerminal() {
				scanner := bufio.NewScanner(os.Stdin)

				// Prompts directory.
				promptsDir := ui.Prompt(scanner, "Prompts directory", "./prompts")
				cfg.Lib.Prompts = promptsDir

				// LLM setup loop.
				if ui.Confirm(scanner, "Configure an LLM provider?") {
					for {
						llm := configureLLM(scanner)
						cfg.LLMs = append(cfg.LLMs, llm)
						if !ui.Confirm(scanner, "Add another LLM?") {
							break
						}
					}
				}
			}

			// Create prompts directory.
			promptsAbs := filepath.Join(cwd, cfg.Lib.Prompts)
			if err := os.MkdirAll(promptsAbs, 0755); err != nil {
				return fmt.Errorf("creating prompts directory: %w", err)
			}

			if err := config.SaveConfig(dest, cfg); err != nil {
				return err
			}
			fmt.Printf("Created %s\n", dest)
			return nil
		},
	}
}

// configureLLM interactively collects LLM configuration from the user.
func configureLLM(scanner *bufio.Scanner) config.LLMConfig {
	llm := config.LLMConfig{}

	llm.Name = ui.Prompt(scanner, "Name (label for this LLM)", "")

	provider := ui.Choose(scanner, "Provider", []string{"anthropic", "openai", "gemini", "custom"})
	if provider == "custom" {
		llm.BaseURL = ui.Prompt(scanner, "Base URL", "")
		llm.Schema = ui.Choose(scanner, "Schema (wire format)", []string{"openai", "anthropic", "gemini"})
	} else {
		llm.Provider = provider
	}

	llm.Model = ui.Prompt(scanner, "Model", "")

	role := ui.Choose(scanner, "Role", []string{"plan", "code", "none"})
	if role != "none" {
		llm.Role = role
	}

	tokStr := ui.Prompt(scanner, "Max tokens", "4096")
	if n, err := strconv.Atoi(tokStr); err == nil {
		llm.MaxTokens = n
	} else {
		llm.MaxTokens = 4096
	}

	return llm
}

