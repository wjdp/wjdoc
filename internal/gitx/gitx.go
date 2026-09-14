// Package gitx wraps the read-only git commands wj needs, degrading quietly
// when git is absent or the project is not a repository.
package gitx

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"
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

// Commit is one commit touching a document.
type Commit struct {
	Hash    string
	Short   string
	Date    time.Time
	Subject string
}

// ModifiedDates maps each of paths (relative to root, slash separated) to the
// date of the most recent commit touching it. It reads the whole docs/ history
// in one git invocation rather than one per file. Outside a repository, or
// without git, it returns an empty map and no error.
func ModifiedDates(root string, paths []string) (map[string]time.Time, error) {
	dates := map[string]time.Time{}
	if len(paths) == 0 {
		return dates, nil
	}

	wanted := make(map[string]bool, len(paths))
	for _, path := range paths {
		wanted[path] = true
	}

	log, err := Run(root, "-c", "core.quotePath=false", "log",
		"--format=%x00%cI", "--name-only", "--date-order", "--relative", "--", "docs/")
	if err != nil {
		return map[string]time.Time{}, nil
	}

	var current time.Time
	for _, line := range strings.Split(log, "\n") {
		if strings.HasPrefix(line, "\x00") {
			current, err = time.Parse(time.RFC3339, strings.TrimSpace(line[1:]))
			if err != nil {
				current = time.Time{}
			}
			continue
		}
		path := strings.TrimSpace(line)
		if path == "" || current.IsZero() || !wanted[path] {
			continue
		}
		if _, seen := dates[path]; !seen {
			dates[path] = current
		}
	}
	return dates, nil
}

// History lists the commits touching path, newest first, following renames.
// Outside a repository it returns nothing.
func History(root, path string) ([]Commit, error) {
	log, err := Run(root, "-c", "core.quotePath=false", "log", "--follow",
		"--format=%H%x00%h%x00%cI%x00%s", "--", path)
	if err != nil {
		return nil, nil
	}

	commits := make([]Commit, 0)
	for _, line := range strings.Split(log, "\n") {
		fields := strings.Split(line, "\x00")
		if len(fields) != 4 {
			continue
		}
		date, err := time.Parse(time.RFC3339, fields[2])
		if err != nil {
			continue
		}
		commits = append(commits, Commit{Hash: fields[0], Short: fields[1], Date: date, Subject: fields[3]})
	}
	return commits, nil
}

var statusPattern = regexp.MustCompile(`(?m)^status:\s*(.+?)\s*$`)

// StatusAt reads the frontmatter status of path as it stood at hash. A file
// that did not exist, or carried no status, yields the empty string.
func StatusAt(root, hash, path string) (string, error) {
	source, err := Run(root, "show", hash+":"+path)
	if err != nil {
		return "", nil
	}
	match := statusPattern.FindStringSubmatch(leadingFence(source))
	if match == nil {
		return "", nil
	}
	return strings.ToLower(strings.Trim(strings.TrimSpace(match[1]), `"'`)), nil
}

// leadingFence returns the YAML between a leading "---" fence and the next
// "---" line, or the empty string when the source has no frontmatter.
func leadingFence(source string) string {
	rest, found := strings.CutPrefix(source, "---\n")
	if !found {
		return ""
	}
	fence, _, found := strings.Cut(rest, "\n---")
	if !found {
		return ""
	}
	return fence
}
