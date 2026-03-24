package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyDir(t *testing.T) {
	src := t.TempDir()

	// Create a file at the root level.
	if err := os.WriteFile(filepath.Join(src, "root.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create a sub-directory with a file.
	subDir := filepath.Join(src, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "copy")

	if err := CopyDir(src, dst); err != nil {
		t.Fatalf("CopyDir: %v", err)
	}

	// Verify root.txt
	got, err := os.ReadFile(filepath.Join(dst, "root.txt"))
	if err != nil {
		t.Fatalf("root.txt not found in dst: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("root.txt: got %q, want %q", got, "hello")
	}

	// Verify sub/nested.txt
	got, err = os.ReadFile(filepath.Join(dst, "sub", "nested.txt"))
	if err != nil {
		t.Fatalf("sub/nested.txt not found in dst: %v", err)
	}
	if string(got) != "world" {
		t.Errorf("sub/nested.txt: got %q, want %q", got, "world")
	}
}
