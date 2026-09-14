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

// Extract returns every link to another file in the repository, skipping
// fenced code blocks, inline code spans, absolute URLs and bare fragments.
// Targets are %-decoded for spaces and stripped of any "#fragment".
func Extract(body string) []Link {
	var found []Link
	fence := ""

	for index, line := range strings.Split(body, "\n") {
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
		outsideCode := stripCodeSpans(trimmed)

		if match := referencePattern.FindStringSubmatch(outsideCode); match != nil {
			if target, ok := cleanTarget(match[1]); ok {
				found = append(found, Link{Target: target, Line: lineNumber})
			}
			continue
		}
		for _, match := range inlinePattern.FindAllStringSubmatch(outsideCode, -1) {
			if target, ok := cleanTarget(match[1]); ok {
				found = append(found, Link{Target: target, Line: lineNumber})
			}
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

// stripCodeSpans blanks out `code` runs so that links inside them are ignored.
func stripCodeSpans(line string) string {
	var out strings.Builder
	rest := line
	for {
		open := strings.IndexByte(rest, '`')
		if open < 0 {
			out.WriteString(rest)
			return out.String()
		}
		out.WriteString(rest[:open])
		end := strings.IndexByte(rest[open+1:], '`')
		if end < 0 {
			return out.String()
		}
		rest = rest[open+1+end+1:]
	}
}
