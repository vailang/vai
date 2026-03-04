package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/vailang/vai/internal/config"
)

func configCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage vai configuration",
	}
	cmd.AddCommand(configLLMCommand())
	return cmd
}

func configLLMCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "llm",
		Short: "Manage LLM provider entries",
	}
	cmd.AddCommand(configLLMListCommand())
	cmd.AddCommand(configLLMAddCommand())
	cmd.AddCommand(configLLMRemoveCommand())
	return cmd
}

func configLLMListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured LLM providers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, _, err := loadProjectConfig()
			if err != nil {
				return err
			}
			if len(cfg.LLMs) == 0 {
				fmt.Println("No LLM providers configured.")
				return nil
			}
			if jsonFlag {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(cfg.LLMs)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tPROVIDER\tMODEL\tROLE\tMAX_TOKENS")
			for _, l := range cfg.LLMs {
				name := l.Name
				if name == "" {
					name = "-"
				}
				role := l.Role
				if role == "" {
					role = "-"
				}
				prov := l.Provider
				if prov == "" && l.BaseURL != "" {
					prov = "custom"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n", name, prov, l.Model, role, l.MaxTokens)
			}
			return w.Flush()
		},
	}
}

func configLLMAddCommand() *cobra.Command {
	var (
		name       string
		prov       string
		model      string
		role       string
		maxTokens  int
		baseURL    string
		schema     string
		envToken   string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add an LLM provider entry",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if model == "" {
				return fmt.Errorf("--model is required")
			}
			if prov == "" && baseURL == "" {
				return fmt.Errorf("--provider or --base-url is required")
			}
			if err := config.ValidateRole(role); err != nil {
				return err
			}

			cfg, cfgPath, err := loadProjectConfig()
			if err != nil {
				return err
			}

			// Validate no duplicate role.
			if role != "" {
				for _, l := range cfg.LLMs {
					if l.Role == role {
						label := l.Name
						if label == "" {
							label = l.Model
						}
						return fmt.Errorf("role %q is already assigned to %q", role, label)
					}
				}
			}

			entry := config.LLMConfig{
				Name:                 name,
				Role:                 role,
				Provider:             prov,
				Schema:               schema,
				Model:                model,
				BaseURL:              baseURL,
				EnvTokenVariableName: envToken,
				MaxTokens:            maxTokens,
			}
			cfg.LLMs = append(cfg.LLMs, entry)

			if err := config.SaveConfig(cfgPath, cfg); err != nil {
				return err
			}

			label := name
			if label == "" {
				label = model
			}
			fmt.Printf("Added LLM %q\n", label)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Human-readable label for this entry")
	cmd.Flags().StringVar(&prov, "provider", "", "Provider: anthropic, openai, gemini")
	cmd.Flags().StringVar(&model, "model", "", "Model identifier (required)")
	cmd.Flags().StringVar(&role, "role", "", "Role: plan, code, or empty")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 4096, "Maximum response tokens")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "Custom API endpoint")
	cmd.Flags().StringVar(&schema, "schema", "", "Wire format for custom providers (openai, anthropic, gemini)")
	cmd.Flags().StringVar(&envToken, "env-token", "", "Environment variable name for the API token")

	return cmd
}

func configLLMRemoveCommand() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove an LLM provider entry by name",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			cfg, cfgPath, err := loadProjectConfig()
			if err != nil {
				return err
			}

			found := false
			var remaining []config.LLMConfig
			for _, l := range cfg.LLMs {
				if strings.EqualFold(l.Name, name) {
					found = true
					continue
				}
				remaining = append(remaining, l)
			}
			if !found {
				return fmt.Errorf("no LLM with name %q found", name)
			}
			cfg.LLMs = remaining

			if err := config.SaveConfig(cfgPath, cfg); err != nil {
				return err
			}

			fmt.Printf("Removed LLM %q\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name of the LLM entry to remove (required)")
	return cmd
}

// loadProjectConfig finds and loads the vai.toml config, returning
// the parsed config and its absolute path.
func loadProjectConfig() (*config.Config, string, error) {
	cfg, cfgPath, _, err := loadProject()
	return cfg, cfgPath, err
}
