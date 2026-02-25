package locker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const lockFileName = "vai.lock"

// LockEntry represents a single locked request.
type LockEntry struct {
	Hash string `json:"hash"`
}

// LockFile represents the vai.lock file contents.
type LockFile struct {
	Version int                  `json:"version"`
	Entries map[string]LockEntry `json:"entries,omitempty"`
}

// Locker manages the vai.lock file, providing thread-safe hash-based
// skip detection for runner requests.
type Locker struct {
	mu   sync.Mutex
	path string
	data LockFile
}

// New creates a Locker rooted at baseDir.
// Loads an existing vai.lock if present; starts fresh otherwise.
func New(baseDir string) *Locker {
	l := &Locker{
		path: filepath.Join(baseDir, lockFileName),
		data: LockFile{
			Version: 1,
			Entries: make(map[string]LockEntry),
		},
	}
	l.load()
	return l
}

// IsLocked returns true if the entry for key exists and its hash matches.
func (l *Locker) IsLocked(key, hash string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.data.Entries[key]
	return ok && entry.Hash == hash
}

// Lock records a hash for the given key.
func (l *Locker) Lock(key, hash string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.data.Entries[key] = LockEntry{Hash: hash}
}

// Save writes the lock file to disk as indented JSON.
func (l *Locker) Save() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	buf, err := json.MarshalIndent(l.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(l.path, buf, 0644)
}

// load reads an existing vai.lock file. Silently starts fresh on any error.
func (l *Locker) load() {
	buf, err := os.ReadFile(l.path)
	if err != nil {
		return
	}
	var lf LockFile
	if err := json.Unmarshal(buf, &lf); err != nil {
		return
	}
	if lf.Entries == nil {
		lf.Entries = make(map[string]LockEntry)
	}
	l.data = lf
}
