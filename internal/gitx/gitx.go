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

// DocNamesAcrossBranches lists the filenames anywhere under docs/ on every local and
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
		listing, err := Run(root, "ls-tree", "-r", "--name-only", ref, "docs/")
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
	// Path is where the document was in this commit, which differs from its
	// path today when it has since been renamed or moved.
	Path string
}

// Dates are the git-derived lifetimes of a document: the commit that first
// introduced it under any name, and the most recent commit to change its
// content. A pure rename is not a content change.
type Dates struct {
	Created  time.Time
	Modified time.Time
}

// DocumentDates maps each of paths (relative to root, slash separated) to its
// Dates. It reads the whole docs/ history in one git invocation rather than one
// per file, following renames so that a document keeps the age of its earlier
// names. Outside a repository, or without git, it returns an empty map and no
// error.
func DocumentDates(root string, paths []string) (map[string]Dates, error) {
	dates := map[string]Dates{}
	if len(paths) == 0 {
		return dates, nil
	}

	wanted := make(map[string]bool, len(paths))
	for _, path := range paths {
		wanted[path] = true
	}

	log, err := Run(root, "-c", "core.quotePath=false", "log",
		"--format=%x00%cI", "--name-status", "-M", "--date-order", "--relative", "--", "docs/")
	if err != nil {
		return map[string]Dates{}, nil
	}

	// renamedTo maps a path as it was known earlier in history to the path the
	// same document has today. The log is walked newest first, so a rename is
	// always seen before the commits that used the older name.
	renamedTo := map[string]string{}
	resolve := func(path string) (string, bool) {
		if today, renamed := renamedTo[path]; renamed {
			path = today
		}
		if !wanted[path] {
			return "", false
		}
		return path, true
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

		fields := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(fields) < 2 || current.IsZero() {
			continue
		}
		change, path := fields[0], fields[1]
		if strings.HasPrefix(change, "R") && len(fields) >= 3 {
			path = fields[2]
			if today, ok := resolve(path); ok {
				renamedTo[fields[1]] = today
			}
		}

		name, ok := resolve(path)
		if !ok {
			continue
		}
		entry := dates[name]
		entry.Created = current
		if entry.Modified.IsZero() && changesContent(change) {
			entry.Modified = current
		}
		dates[name] = entry
	}
	return dates, nil
}

// changesContent reports whether a --name-status change letter touched the
// file's bytes. R100 is a pure rename and leaves the content alone.
func changesContent(change string) bool {
	if change == "R100" {
		return false
	}
	return strings.HasPrefix(change, "A") || strings.HasPrefix(change, "M") || strings.HasPrefix(change, "R")
}

// History lists the commits touching path, newest first, following renames.
// Outside a repository it returns nothing.
func History(root, path string) ([]Commit, error) {
	log, err := Run(root, "-c", "core.quotePath=false", "log", "--follow", "--name-only",
		"--format=%x00%H%x00%h%x00%cI%x00%s", "--", path)
	if err != nil {
		return nil, nil
	}

	commits := make([]Commit, 0)
	for _, line := range strings.Split(log, "\n") {
		if !strings.HasPrefix(line, "\x00") {
			if name := strings.TrimSpace(line); name != "" && len(commits) > 0 && commits[len(commits)-1].Path == "" {
				commits[len(commits)-1].Path = name
			}
			continue
		}
		fields := strings.Split(line[1:], "\x00")
		if len(fields) != 4 {
			continue
		}
		date, err := time.Parse(time.RFC3339, fields[2])
		if err != nil {
			continue
		}
		commits = append(commits, Commit{Hash: fields[0], Short: fields[1], Date: date, Subject: fields[3]})
	}
	for index := range commits {
		if commits[index].Path == "" {
			commits[index].Path = path
		}
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
