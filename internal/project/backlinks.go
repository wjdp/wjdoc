// Package project holds the whole-repository operations: the backlink graph
// and the one-off setup wj init performs.
package project

import (
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/wjdp/wjdoc/internal/docs"
	"github.com/wjdp/wjdoc/internal/links"
)

// Reference is one place a document is mentioned, with the 1-based line in
// that file.
type Reference struct {
	Path string
	Line int
}

// Backlinks lists every mention of target: relative markdown links from the
// other documents, and mentions of its path or filename anywhere else in the
// repository. The repository scan uses ripgrep and is skipped, without an
// error, when ripgrep is not installed.
func Backlinks(root string, target docs.Document, documents []docs.Document) ([]Reference, error) {
	found := documentReferences(target, documents)

	outside, err := repositoryReferences(root, target)
	if err != nil {
		return nil, err
	}
	return sortedUnique(append(found, outside...)), nil
}

func documentReferences(target docs.Document, documents []docs.Document) []Reference {
	var found []Reference
	for _, document := range documents {
		if document.Path == target.Path {
			continue
		}
		offset := BodyOffset(document)
		for _, link := range links.Extract(document.Body) {
			if links.ResolveTarget(document.Path, link.Target) == target.Path {
				found = append(found, Reference{Path: document.Path, Line: link.Line + offset})
			}
		}
	}
	return found
}

// repositoryReferences greps the repository outside docs/ for the target's
// path and bare filename.
func repositoryReferences(root string, target docs.Document) ([]Reference, error) {
	if _, err := exec.LookPath("rg"); err != nil {
		return nil, nil
	}

	filename := pathBase(target.Path)
	command := exec.Command("rg",
		"--line-number", "--no-heading", "--fixed-strings",
		"--glob", "!docs/**",
		"-e", filename, "-e", target.Path)
	command.Dir = root

	output, err := command.Output()
	if err != nil && len(output) == 0 {
		return nil, nil
	}

	var found []Reference
	for _, line := range strings.Split(string(output), "\n") {
		if reference, ok := parseGrepLine(line); ok {
			found = append(found, reference)
		}
	}
	return found, nil
}

func parseGrepLine(line string) (Reference, bool) {
	path, rest, found := strings.Cut(strings.TrimRight(line, "\r"), ":")
	if !found || path == "" {
		return Reference{}, false
	}
	number, _, found := strings.Cut(rest, ":")
	if !found {
		return Reference{}, false
	}
	lineNumber, err := strconv.Atoi(number)
	if err != nil {
		return Reference{}, false
	}
	path = strings.TrimPrefix(path, "./")
	if path == "" || strings.HasPrefix(path, "docs/") {
		return Reference{}, false
	}
	return Reference{Path: path, Line: lineNumber}, true
}

// BodyOffset is the number of lines the frontmatter block takes up, so that a
// line number within Document.Body can be turned into a line in the file.
func BodyOffset(document docs.Document) int {
	source, err := os.ReadFile(document.AbsPath)
	if err != nil {
		return 0
	}

	lines := strings.Split(string(source), "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r") != "---" {
		return 0
	}
	for index, line := range lines[1:] {
		if strings.TrimRight(line, "\r") == "---" {
			return index + 2
		}
	}
	return 0
}

// FileLine turns a 1-based line within a document's body into its line in the
// file on disk.
func FileLine(document docs.Document, bodyLine int) int {
	return bodyLine + BodyOffset(document)
}

func sortedUnique(references []Reference) []Reference {
	seen := map[Reference]bool{}
	unique := make([]Reference, 0, len(references))
	for _, reference := range references {
		if seen[reference] {
			continue
		}
		seen[reference] = true
		unique = append(unique, reference)
	}

	sort.Slice(unique, func(i, j int) bool {
		if unique[i].Path == unique[j].Path {
			return unique[i].Line < unique[j].Line
		}
		return unique[i].Path < unique[j].Path
	})
	return unique
}

func pathBase(path string) string {
	if index := strings.LastIndex(path, "/"); index >= 0 {
		return path[index+1:]
	}
	return path
}
