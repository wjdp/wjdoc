// Package gitx wraps the read-only git commands wj needs, degrading quietly
// when git is absent or the project is not a repository.
package gitx

import (
	"bytes"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// Run executes git in root and returns its trimmed standard output. Standard
// error is folded into the returned error.
func Run(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	command.Stdout = stdout
	command.Stderr = stderr

	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, message)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// DocNamesAcrossBranches lists the filenames under docs/ on every local and
// remote branch, so that numbers allocated on another branch are not reused.
// It returns nil without an error when git is unavailable or root is not a
// repository: number allocation still works, it just sees less.
func DocNamesAcrossBranches(root string) ([]string, error) {
	refs, err := Run(root, "for-each-ref", "--format=%(refname)", "refs/heads", "refs/remotes")
	if err != nil {
		return nil, nil
	}

	seen := map[string]bool{}
	for _, ref := range strings.Split(refs, "\n") {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		listing, err := Run(root, "ls-tree", "--name-only", ref, "docs/")
		if err != nil {
			continue
		}
		for _, path := range strings.Split(listing, "\n") {
			path = strings.TrimSpace(path)
			if path == "" {
				continue
			}
			seen[path[strings.LastIndex(path, "/")+1:]] = true
		}
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}
