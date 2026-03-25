package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CreateLink creates a directory junction on Windows (does not require admin
// rights, unlike symlinks). Falls back to CopyDir if link and target are on
// different drives.
func CreateLink(link, target string) error {
	// Determine drive letters to detect cross-drive scenario.
	linkVol := filepath.VolumeName(filepath.Clean(link))
	targetVol := filepath.VolumeName(filepath.Clean(target))
	if strings.EqualFold(linkVol, targetVol) {
		// Pass mklink args individually. cmd /C joins them with spaces.
		// Account names are validated to [a-zA-Z0-9_-]+ so paths are safe.
		cmd := exec.Command("cmd", "/C", "mklink", "/J", link, target)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("mklink /J: %w\n%s", err, out)
		}
		return nil
	}
	// Cross-drive: fall back to a full recursive copy.
	return CopyDir(target, link)
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
