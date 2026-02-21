package reader

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// FileSource implements lexer.CodeSource for .vai/.plan file content.
type FileSource struct {
	code string
}

func (v *FileSource) GetCode() string { return v.code }
func (v *FileSource) GetOffset() uint { return 0 }
func (v *FileSource) IsVaiCode() bool { return true }

// NewVaiSource creates a FileSource from a plain .vai file string.
func NewVaiSource(code string) *FileSource {
	return &FileSource{code: code}
}

// ReadPaths accepts a path (file or directory) and returns a map of
// absolute file paths to their contents. Only .vai and .plan files are
// included. Directories are walked recursively; hidden directories
// (names starting with '.') are skipped.
func ReadPaths(root string) (map[string]string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolving path %s: %w", root, err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", abs, err)
	}

	files := make(map[string]string)

	if !info.IsDir() {
		if !isVaiFile(abs) {
			return nil, fmt.Errorf("%s: not a .vai or .plan file", abs)
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", abs, err)
		}
		files[abs] = string(data)
		return files, nil
	}

	err = filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") && path != abs {
				return filepath.SkipDir
			}
			return nil
		}
		if isVaiFile(path) {
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("reading %s: %w", path, err)
			}
			files[path] = string(data)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

// isVaiFile reports whether path has a .vai or .plan extension.
func isVaiFile(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".vai" || ext == ".plan"
}
