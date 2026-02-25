package locker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLockerEmptyStart(t *testing.T) {
	dir := t.TempDir()
	l := New(dir)

	if l.IsLocked("plan:test", "abc123") {
		t.Error("new locker should not have any locked entries")
	}
}

func TestLockAndCheck(t *testing.T) {
	dir := t.TempDir()
	l := New(dir)

	l.Lock("plan:myplan", "hash1")
	if !l.IsLocked("plan:myplan", "hash1") {
		t.Error("should be locked after Lock()")
	}

	// Different hash should not match.
	if l.IsLocked("plan:myplan", "hash2") {
		t.Error("different hash should not match")
	}

	// Different key should not match.
	if l.IsLocked("plan:other", "hash1") {
		t.Error("different key should not match")
	}
}

func TestSaveAndReload(t *testing.T) {
	dir := t.TempDir()

	// Create and save.
	l1 := New(dir)
	l1.Lock("plan:rust", "abc")
	l1.Lock("impl:rust.add", "def")
	if err := l1.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify file exists.
	lockPath := filepath.Join(dir, "vai.lock")
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("vai.lock should exist: %v", err)
	}

	// Reload.
	l2 := New(dir)
	if !l2.IsLocked("plan:rust", "abc") {
		t.Error("plan:rust should be locked after reload")
	}
	if !l2.IsLocked("impl:rust.add", "def") {
		t.Error("impl:rust.add should be locked after reload")
	}
	if l2.IsLocked("plan:rust", "changed") {
		t.Error("changed hash should not match after reload")
	}
}

func TestCorruptedLockFile(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "vai.lock")

	// Write invalid JSON.
	if err := os.WriteFile(lockPath, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	// Should start fresh without error.
	l := New(dir)
	if l.IsLocked("anything", "hash") {
		t.Error("corrupted lock file should result in empty state")
	}

	// Should be able to save over it.
	l.Lock("key", "val")
	if err := l.Save(); err != nil {
		t.Fatalf("Save() over corrupted file should work: %v", err)
	}
}

func TestOverwriteEntry(t *testing.T) {
	dir := t.TempDir()
	l := New(dir)

	l.Lock("plan:test", "old")
	l.Lock("plan:test", "new")

	if l.IsLocked("plan:test", "old") {
		t.Error("old hash should be overwritten")
	}
	if !l.IsLocked("plan:test", "new") {
		t.Error("new hash should be active")
	}
}
