package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Package describes a discovered vai package defined by a vai.toml file.
type Package struct {
	Name       string     // lib.name from vai.toml
	ConfigPath string     // absolute path to vai.toml
	RootDir    string     // directory containing vai.toml
	SrcDir     string     // absolute path to lib.prompts directory
	Config     *Config
	Children   []*Package
}

// DiscoverTree finds the root vai.toml (walking upward from startDir)
// and recursively discovers all nested sub-packages.
func DiscoverTree(startDir string) (*Package, error) {
	cfgPath, err := FindConfig(startDir)
	if err != nil {
		return nil, err
	}
	return loadPackageTree(cfgPath)
}

// loadPackageTree loads a package from a vai.toml path and discovers children.
func loadPackageTree(cfgPath string) (*Package, error) {
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return nil, err
	}

	rootDir := filepath.Dir(cfgPath)
	srcDir := filepath.Join(rootDir, cfg.Lib.Prompts)
	srcDir, err = filepath.Abs(srcDir)
	if err != nil {
		return nil, err
	}

	pkg := &Package{
		Name:       cfg.Lib.Name,
		ConfigPath: cfgPath,
		RootDir:    rootDir,
		SrcDir:     srcDir,
		Config:     cfg,
	}

	// Walk the root directory (not just src) to find nested vai.toml files.
	children, err := discoverChildren(rootDir, cfgPath)
	if err != nil {
		return nil, err
	}
	pkg.Children = children

	return pkg, nil
}

// discoverChildren walks dir looking for nested vai.toml files,
// skipping the parent's own vai.toml and hidden directories.
func discoverChildren(dir string, parentCfgPath string) ([]*Package, error) {
	var children []*Package

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Skip hidden directories.
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		subDir := filepath.Join(dir, entry.Name())
		candidate := filepath.Join(subDir, "vai.toml")

		if _, err := os.Stat(candidate); err == nil {
			// Found a sub-package — load it recursively.
			child, err := loadPackageTree(candidate)
			if err != nil {
				return nil, fmt.Errorf("loading sub-package %s: %w", candidate, err)
			}
			children = append(children, child)
		} else {
			// No vai.toml here — keep looking deeper.
			deeper, err := discoverChildren(subDir, parentCfgPath)
			if err != nil {
				return nil, err
			}
			children = append(children, deeper...)
		}
	}

	return children, nil
}

// LoadPackageFiles collects all .vai and .plan files belonging to a package.
// It reads from the package's SrcDir, stopping at subdirectories that
// contain their own vai.toml (sub-package boundaries).
func LoadPackageFiles(pkg *Package) (map[string]string, error) {
	files := make(map[string]string)

	// Also collect files from RootDir itself (not just src).
	if err := collectFiles(pkg.RootDir, files); err != nil {
		return nil, err
	}

	// Collect from SrcDir if it differs from RootDir.
	if pkg.SrcDir != pkg.RootDir {
		if err := walkForFiles(pkg.SrcDir, files); err != nil {
			return nil, err
		}
	}

	return files, nil
}

// collectFiles reads .vai/.plan files from a single directory (non-recursive).
func collectFiles(dir string, files map[string]string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext == ".vai" || ext == ".plan" {
			path := filepath.Join(dir, entry.Name())
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files[path] = string(content)
		}
	}
	return nil
}

// walkForFiles recursively collects .vai/.plan files, stopping at sub-package boundaries.
func walkForFiles(dir string, files map[string]string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			// Stop at sub-package boundaries.
			if _, err := os.Stat(filepath.Join(path, "vai.toml")); err == nil {
				continue
			}
			if err := walkForFiles(path, files); err != nil {
				return err
			}
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext == ".vai" || ext == ".plan" {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files[path] = string(content)
		}
	}
	return nil
}
