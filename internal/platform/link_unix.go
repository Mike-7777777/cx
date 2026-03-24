//go:build !windows

package platform

import (
	"os"
	"path/filepath"
)

// CreateLink creates a symlink on Unix.
func CreateLink(link, target string) error {
	return os.Symlink(target, link)
}

// CopyDir recursively copies src into dst, creating dst if it does not exist.
func CopyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}
		return copyFile(path, destPath)
	})
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
