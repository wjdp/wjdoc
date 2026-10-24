// Package lint checks a project's documents against the docs/ convention.
package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/wjdp/wjdoc/internal/docs"
	"github.com/wjdp/wjdoc/internal/links"
)

// Level is the severity of a finding. Errors fail a lint run, warnings do not.
type Level int

const (
	Error Level = iota
	Warning
)

func (l Level) String() string {
	if l == Warning {
		return "warning"
	}
	return "error"
}

// Finding is one problem with one document. Line is 0 when the problem belongs
// to the file as a whole rather than to a line of it.
type Finding struct {
	Path    string
	Line    int
	Level   Level
	Message string
}

var (
	filenamePattern = regexp.MustCompile(`^\d+-[A-Z0-9].*\.md$`)
	yamlLinePattern = regexp.MustCompile(`\[(\d+):\d+\]|line (\d+)`)
)

// Run applies every convention rule to documents and returns the findings,
// sorted by path and then by line.
func Run(root string, documents []docs.Document) []Finding {
	var findings []Finding
	add := func(document docs.Document, line int, level Level, format string, args ...any) {
		findings = append(findings, Finding{
			Path:    document.Path,
			Line:    line,
			Level:   level,
			Message: fmt.Sprintf(format, args...),
		})
	}

	projectWidth := docs.Width(documents)
	byPath := map[string]docs.Document{}
	for _, document := range documents {
		byPath[document.Path] = document
	}

	for _, document := range documents {
		name := filepath.Base(document.Path)
		if !filenamePattern.MatchString(name) {
			add(document, 0, Error, "filename does not follow NNN-Sentence-case-title.md")
		}
		if strings.ContainsAny(name, " \t") {
			add(document, 0, Error, "filename contains spaces, slugs use hyphens")
		}
		if document.Number >= 0 && document.Width != projectWidth {
			add(document, 0, Warning, "padding width %d differs from the project width of %d", document.Width, projectWidth)
		}
		for _, other := range duplicates(documents, document) {
			add(document, 0, Error, "number %d is also used by %s", document.Number, other)
		}

		switch {
		case document.Err != nil:
			add(document, yamlLine(document.Err), Error, "frontmatter is not valid YAML: %s", firstLine(document.Err, document.Path))
		case !hasFrontmatter(document):
			add(document, 1, Error, "frontmatter is missing")
		default:
			findings = append(findings, frontmatterFindings(document)...)
			findings = append(findings, priorityFindings(document)...)
			findings = append(findings, tagFindings(document)...)
		}

		findings = append(findings, linkFindings(root, byPath, document)...)
	}

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		return findings[i].Line < findings[j].Line
	})
	return findings
}

// HasErrors reports whether any finding is an error.
func HasErrors(findings []Finding) bool {
	for _, finding := range findings {
		if finding.Level == Error {
			return true
		}
	}
	return false
}

func frontmatterFindings(document docs.Document) []Finding {
	var findings []Finding
	add := func(level Level, format string, args ...any) {
		findings = append(findings, Finding{
			Path:    document.Path,
			Line:    1,
			Level:   level,
			Message: fmt.Sprintf(format, args...),
		})
	}

	if document.Type == "" {
		add(Error, "type is missing")
		return findings
	}
	if !contains(docs.KnownTypes, document.Type) {
		add(Warning, "unknown type %q", document.Type)
		return findings
	}

	if document.Status != "" && !contains(docs.KnownStatuses, document.Status) {
		add(Warning, "unknown status %q", document.Status)
	}
	if types, narrowed := docs.StatusOnlyFor[document.Status]; narrowed && !contains(types, document.Type) {
		add(Warning, "status %q is only for %s", document.Status, strings.Join(types, " and "))
	}
	switch {
	case docs.StatusRequired(document.Type) && document.Status == "":
		add(Error, "status is missing, required for a %s", document.Type)
	case !docs.StatusRequired(document.Type) && document.Status != "":
		add(Warning, "status is not used on a %s", document.Type)
	}
	return findings
}

// frontmatterFinding is a finding against the frontmatter block as a whole.
func frontmatterFinding(document docs.Document, level Level, format string, args ...any) Finding {
	return Finding{Path: document.Path, Line: 1, Level: level, Message: fmt.Sprintf(format, args...)}
}

func priorityFindings(document docs.Document) []Finding {
	value, present := document.Frontmatter["priority"]
	if !present || value == nil {
		return nil
	}
	switch value.(type) {
	case []any, map[string]any:
		return []Finding{frontmatterFinding(document, Error, "priority is not a single value")}
	}

	var findings []Finding
	if !contains(docs.KnownPriorities, document.Priority) {
		findings = append(findings, frontmatterFinding(document, Warning, "unknown priority %q, want one of %s", document.Priority, strings.Join(docs.KnownPriorities, ", ")))
	}
	if document.Type == "reference" {
		findings = append(findings, frontmatterFinding(document, Warning, "priority is not used on a reference"))
	}
	return findings
}

func tagFindings(document docs.Document) []Finding {
	value, present := document.Frontmatter["tags"]
	if !present || value == nil {
		return nil
	}
	if document.Tags == nil {
		return []Finding{frontmatterFinding(document, Error, "tags is not a list of strings, write tags: [%v]", value)}
	}

	var findings []Finding
	seen := map[string]bool{}
	for _, tag := range document.Tags {
		if !docs.TagPattern.MatchString(tag) {
			findings = append(findings, frontmatterFinding(document, Warning, "tag %q is not a lowercase name or namespace:value", tag))
		}
		if seen[tag] {
			findings = append(findings, frontmatterFinding(document, Warning, "tag %q is repeated", tag))
		}
		seen[tag] = true
	}
	return findings
}

func linkFindings(root string, byPath map[string]docs.Document, document docs.Document) []Finding {
	var findings []Finding
	for _, link := range links.Extract(document.Body) {
		target := links.ResolveTarget(document.Path, link.Target)
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(target))); err != nil {
			findings = append(findings, Finding{
				Path:    document.Path,
				Line:    link.Line,
				Level:   Error,
				Message: fmt.Sprintf("link target %s does not exist", target),
			})
			continue
		}
		if linked, known := byPath[target]; known && linked.Status == "superseded" && !docs.Inactive(document.Status) {
			findings = append(findings, Finding{
				Path:    document.Path,
				Line:    link.Line,
				Level:   Warning,
				Message: fmt.Sprintf("links to %s, which is superseded", target),
			})
		}
	}
	return findings
}

func duplicates(documents []docs.Document, document docs.Document) []string {
	if document.Number < 0 {
		return nil
	}
	var others []string
	for _, candidate := range documents {
		if candidate.Number == document.Number && candidate.Path != document.Path {
			others = append(others, candidate.Path)
		}
	}
	return others
}

// hasFrontmatter reports whether the file opens with a "---" fence; the loader
// leaves no trace of the difference between absent and empty frontmatter.
func hasFrontmatter(document docs.Document) bool {
	source, err := os.ReadFile(document.AbsPath)
	if err != nil {
		return true
	}
	first, _, _ := strings.Cut(string(source), "\n")
	return strings.TrimRight(first, "\r") == "---"
}

func yamlLine(err error) int {
	match := yamlLinePattern.FindStringSubmatch(err.Error())
	if match == nil {
		return 1
	}
	for _, group := range match[1:] {
		if line, convertErr := strconv.Atoi(group); convertErr == nil && line > 0 {
			return line
		}
	}
	return 1
}

// firstLine is the error's headline, without the loader's path prefix or the
// YAML decoder's multi-line source excerpt.
func firstLine(err error, path string) string {
	message, _, _ := strings.Cut(err.Error(), "\n")
	return strings.TrimSpace(strings.TrimPrefix(message, path+": "))
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
