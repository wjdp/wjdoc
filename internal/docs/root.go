package docs

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrNoDocsDir is returned when no docs/ directory is found at or above start.
var ErrNoDocsDir = errors.New("no docs/ directory found")

// FindRoot walks up from start looking for a directory containing docs/, and
// failing that the nearest directory containing .git.
func FindRoot(start string) (string, error) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}

	gitRoot := ""
	for {
		if isDir(filepath.Join(current, "docs")) {
			return current, nil
		}
		if gitRoot == "" && exists(filepath.Join(current, ".git")) {
			gitRoot = current
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	if gitRoot != "" {
		return gitRoot, nil
	}
	return "", ErrNoDocsDir
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
