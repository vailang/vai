package prompter

import (
	"os"
	"path/filepath"
)

// ChangeType describes whether a file was created or modified.
type ChangeType string

const (
	ChangeCreated  ChangeType = "created"
	ChangeModified ChangeType = "modified"
)

// Change records a single file change made by the prompter.
type Change struct {
	Path     string     // relative to project root
	Type     ChangeType
	Content  string
	Original string // original content (only for Modified)
	IsVai    bool   // true for .vai/.plan files, false for stubs
}

// Result holds the outcome of a prompter run.
type Result struct {
	Changes   []Change
	Summary   string // LLM's final text response
	TokensIn  int
	TokensOut int
}

// Flush writes all changes to disk relative to baseDir.
func (r *Result) Flush(baseDir string) error {
	for _, ch := range r.Changes {
		absPath := filepath.Join(baseDir, ch.Path)
		if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(absPath, []byte(ch.Content), 0644); err != nil {
			return err
		}
	}
	return nil
}

// Rollback undoes all changes: deletes created files, restores modified ones.
func (r *Result) Rollback(baseDir string) {
	for _, ch := range r.Changes {
		absPath := filepath.Join(baseDir, ch.Path)
		switch ch.Type {
		case ChangeCreated:
			_ = os.Remove(absPath)
		case ChangeModified:
			_ = os.WriteFile(absPath, []byte(ch.Original), 0644)
		}
	}
}
