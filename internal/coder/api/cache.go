package api

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// OverrideDir allows tests to override the cache directory.
var OverrideDir string

// HeaderInfo is the cached representation of a file's extracted headers.
type HeaderInfo struct {
	Hash     string   `json:"hash"`
	Language string   `json:"language"`
	FilePath string   `json:"file_path"`
	Symbols  []Symbol `json:"symbols"`
	CachedAt string   `json:"cached_at"`
}

// CacheDir returns ~/.vai/headers/ (or OverrideDir if set), creating it if needed.
func CacheDir() (string, error) {
	var dir string
	if OverrideDir != "" {
		dir = OverrideDir
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".vai", "headers")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// ComputeHash returns the SHA256 hex digest of source content.
func ComputeHash(source []byte) string {
	h := sha256.Sum256(source)
	return fmt.Sprintf("%x", h)
}

// cacheFileName returns a deterministic filename for a given file path.
func cacheFileName(filePath string) string {
	h := sha256.Sum256([]byte(filePath))
	return fmt.Sprintf("%x.json", h)
}

// LoadCache reads cached header info for filePath.
// Returns nil, nil if cache doesn't exist or the content hash doesn't match.
func LoadCache(filePath string, currentHash string) (*HeaderInfo, error) {
	dir, err := CacheDir()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filepath.Join(dir, cacheFileName(filePath)))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var info HeaderInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	if info.Hash != currentHash {
		return nil, nil // stale
	}
	return &info, nil
}

// SaveCache writes header info to the cache directory.
func SaveCache(info *HeaderInfo) error {
	dir, err := CacheDir()
	if err != nil {
		return err
	}

	info.CachedAt = time.Now().UTC().Format(time.RFC3339)

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, cacheFileName(info.FilePath)), data, 0644)
}
