package filemanager

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// fileEntry holds a single file's in-memory state.
type fileEntry struct {
	mu       sync.Mutex
	original []byte
	current  []byte
	dirty    bool
}

// FileManager holds all target files in memory with per-file locking.
// Nothing touches disk until Flush() is called.
type FileManager struct {
	mu    sync.RWMutex
	files map[string]*fileEntry
}

// New creates an empty FileManager.
func New() *FileManager {
	return &FileManager{
		files: make(map[string]*fileEntry),
	}
}

// Load reads a file from disk into memory. If the file does not exist,
// it is initialized with empty content.
func (fm *FileManager) Load(path string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, ok := fm.files[path]; ok {
		return nil // already loaded
	}

	var content []byte
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("loading %s: %w", path, err)
		}
		// File doesn't exist yet — start with empty content.
		content = nil
	} else {
		content = data
	}

	orig := make([]byte, len(content))
	copy(orig, content)
	fm.files[path] = &fileEntry{
		original: orig,
		current:  content,
	}
	return nil
}

// Read returns the in-memory content for a path.
func (fm *FileManager) Read(path string) ([]byte, bool) {
	fm.mu.RLock()
	entry, ok := fm.files[path]
	fm.mu.RUnlock()
	if !ok {
		return nil, false
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	cp := make([]byte, len(entry.current))
	copy(cp, entry.current)
	return cp, true
}

// Write updates the in-memory content for a path.
// If the path was not previously loaded, it is auto-created.
func (fm *FileManager) Write(path string, content []byte) {
	fm.mu.Lock()
	entry, ok := fm.files[path]
	if !ok {
		fm.files[path] = &fileEntry{
			original: nil,
			current:  content,
			dirty:    true,
		}
		fm.mu.Unlock()
		return
	}
	fm.mu.Unlock()

	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.current = content
	entry.dirty = true
}

// Flush writes all dirty files to disk. Creates parent directories as needed.
func (fm *FileManager) Flush() error {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	for path, entry := range fm.files {
		if err := flushEntry(path, entry); err != nil {
			return err
		}
	}
	return nil
}

func flushEntry(path string, entry *fileEntry) error {
	entry.mu.Lock()
	defer entry.mu.Unlock()

	if !entry.dirty {
		return nil
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	if err := os.WriteFile(path, entry.current, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	entry.dirty = false
	return nil
}

// ModifyFile acquires the per-file lock, calls fn with the current content,
// and writes the result back. This ensures atomicity for read-modify-write cycles.
func (fm *FileManager) ModifyFile(path string, fn func(content []byte) ([]byte, error)) error {
	fm.mu.RLock()
	entry, ok := fm.files[path]
	fm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("file not loaded: %s", path)
	}

	entry.mu.Lock()
	defer entry.mu.Unlock()

	newContent, err := fn(entry.current)
	if err != nil {
		return err
	}
	entry.current = newContent
	entry.dirty = true
	return nil
}
