package reader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadPathsSingleVaiFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.vai")
	content := "func greet() { hello }"
	if err := os.WriteFile(path, []byte(content), 0644) ; err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	files, err := ReadPaths(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[path] != content {
		t.Errorf("content mismatch:\ngot:  %q\nwant: %q", files[path], content)
	}
}

func TestReadPathsSinglePlanFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "build.plan")
	content := "plan Build { target \"main.go\" }"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	files, err := ReadPaths(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[path] != content {
		t.Errorf("content mismatch:\ngot:  %q\nwant: %q", files[path], content)
	}
}

func TestReadPathsRejectsNonVaiFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := ReadPaths(path)
	if err == nil {
		t.Fatal("expected error for non-.vai file")
	}
}

func TestReadPathsDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.vai"), []byte("aaa"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.plan"), []byte("bbb"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "c.go"), []byte("ccc"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "d.txt"), []byte("ddd"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	files, err := ReadPaths(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[filepath.Join(dir, "a.vai")] != "aaa" {
		t.Error("missing or wrong content for a.vai")
	}
	if files[filepath.Join(dir, "b.plan")] != "bbb" {
		t.Error("missing or wrong content for b.plan")
	}
}

func TestReadPathsRecursive(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub", "deep")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatalf("failed to create directories: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "root.vai"), []byte("root"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "nested.vai"), []byte("nested"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "nested.plan"), []byte("plan"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	files, err := ReadPaths(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}
}

func TestReadPathsSkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()
	hidden := filepath.Join(dir, ".hidden")
	if err := os.MkdirAll(hidden, 0755); err != nil {
		t.Fatalf("failed to create hidden directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "visible.vai"), []byte("ok"), 0644); err != nil {
		t.Fatalf("failed to write visible file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hidden, "secret.vai"), []byte("hidden"), 0644); err != nil {
		t.Fatalf("failed to write hidden file: %v", err)
	}

	files, err := ReadPaths(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if _, ok := files[filepath.Join(hidden, "secret.vai")]; ok {
		t.Error("should not include files from hidden directories")
	}
}

func TestReadPathsNonExistent(t *testing.T) {
	_, err := ReadPaths("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestReadPathsAbsoluteKeys(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.vai"), []byte("x"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	files, err := ReadPaths(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for k := range files {
		if !filepath.IsAbs(k) {
			t.Errorf("expected absolute path key, got %q", k)
		}
	}
}

func TestNewVaiSource(t *testing.T) {
	src := NewVaiSource("func main() { hello }")
	if src.GetCode() != "func main() { hello }" {
		t.Errorf("unexpected code: %q", src.GetCode())
	}
	if src.GetOffset() != 0 {
		t.Errorf("expected offset 0, got %d", src.GetOffset())
	}
	if !src.IsVaiCode() {
		t.Error("expected IsVaiCode=true")
	}

}
