// Package links extracts the relative markdown links between documents.
package links

import (
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// Link is one relative markdown link, with the 1-based line it appears on.
type Link struct {
	Target string
	Line   int
}

var (
	inlinePattern    = regexp.MustCompile(`\[[^\]]*\]\(([^()]*)\)`)
	referencePattern = regexp.MustCompile(`^ {0,3}\[[^\]]+\]:\s*(\S+)`)
	fencePattern     = regexp.MustCompile("^\\s{0,3}(```+|~~~+)")
	schemePattern    = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
)

// Destination is the raw destination of one markdown link as written, with
// its 1-based line and its byte offsets within that line.
type Destination struct {
	Raw   string
	Line  int
	Start int
	End   int
}

// Destinations returns the destination of every inline link and reference
// definition in source, skipping fenced code blocks and inline code spans.
func Destinations(source string) []Destination {
	var found []Destination
	fence := ""

	for index, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimRight(line, "\r")
		if marker := fencePattern.FindStringSubmatch(trimmed); marker != nil {
			switch {
			case fence == "":
				fence = marker[1][:3]
			case strings.HasPrefix(marker[1], fence):
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}

		lineNumber := index + 1
		outsideCode := blankCodeSpans(trimmed)
		add := func(start, end int) {
			found = append(found, Destination{Raw: trimmed[start:end], Line: lineNumber, Start: start, End: end})
		}

		if match := referencePattern.FindStringSubmatchIndex(outsideCode); match != nil {
			add(match[2], match[3])
			continue
		}
		for _, match := range inlinePattern.FindAllStringSubmatchIndex(outsideCode, -1) {
			add(match[2], match[3])
		}
	}
	return found
}

// Extract returns every link to another file in the repository, skipping
// fenced code blocks, inline code spans, absolute URLs and bare fragments.
// Targets are %-decoded for spaces and stripped of any "#fragment".
func Extract(body string) []Link {
	var found []Link
	for _, destination := range Destinations(body) {
		if target, ok := cleanTarget(destination.Raw); ok {
			found = append(found, Link{Target: target, Line: destination.Line})
		}
	}
	return found
}

// ResolveTarget gives the repository-relative path a link points at, from a
// document at fromPath.
func ResolveTarget(fromPath, target string) string {
	if strings.HasPrefix(target, "/") {
		return path.Clean(strings.TrimPrefix(target, "/"))
	}
	return path.Clean(path.Join(path.Dir(filepath.ToSlash(fromPath)), target))
}

// cleanTarget normalises one raw link destination and reports whether it names
// a file in the repository.
func cleanTarget(raw string) (string, bool) {
	target := strings.TrimSpace(raw)
	target = strings.TrimPrefix(target, "<")
	target = strings.TrimSuffix(target, ">")
	if field, _, found := strings.Cut(target, " "); found {
		target = field
	}
	if fragment := strings.IndexByte(target, '#'); fragment >= 0 {
		target = target[:fragment]
	}
	target = strings.ReplaceAll(target, "%20", " ")
	if target == "" || schemePattern.MatchString(target) || strings.HasPrefix(target, "//") {
		return "", false
	}
	return target, true
}

// blankCodeSpans replaces `code` runs, and everything after an unclosed
// backtick, with spaces, so that links inside them are ignored while every
// other byte keeps its offset.
func blankCodeSpans(line string) string {
	blanked := []byte(line)
	for offset := 0; offset < len(blanked); {
		open := strings.IndexByte(line[offset:], '`')
		if open < 0 {
			break
		}
		open += offset
		end := strings.IndexByte(line[open+1:], '`')
		last := len(line) - 1
		if end >= 0 {
			last = open + 1 + end
		}
		for index := open; index <= last; index++ {
			blanked[index] = ' '
		}
		offset = last + 1
	}
	return string(blanked)
}
