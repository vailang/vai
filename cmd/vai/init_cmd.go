package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
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
			content := fmt.Sprintf("[lib]\nname = %q\nprompts = \"./prompts\"\n", name)
			if err := os.WriteFile(dest, []byte(content), 0644); err != nil {
				return err
			}
			fmt.Printf("Created %s\n", dest)
			return nil
		},
	}
}
