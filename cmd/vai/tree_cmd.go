package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vailang/vai/internal/config"
)

func treeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "tree",
		Short: "Show the package tree",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			root, err := config.DiscoverTree(cwd)
			if err != nil {
				return fmt.Errorf("no vai.toml found (run 'vai init <name>' to create one)")
			}
			printTree(root, cwd, 0)
			return nil
		},
	}
}

func printTree(pkg *config.Package, cwd string, depth int) {
	rel, _ := filepath.Rel(cwd, pkg.ConfigPath)
	indent := strings.Repeat("  ", depth)
	fmt.Printf("%s%s  (%s)\n", indent, pkg.Name, rel)
	for _, child := range pkg.Children {
		printTree(child, cwd, depth+1)
	}
}
