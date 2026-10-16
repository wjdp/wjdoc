// Package docs discovers and parses the numbered markdown documents in a
// project's docs/ directory.
package docs

import "time"

// Document is one numbered markdown file in docs/.
type Document struct {
	// Path is relative to the repository root, e.g. "docs/007-Sync-robustness.md".
	Path string
	// AbsPath is the absolute filesystem path.
	AbsPath string
	// Number is the document's identity, parsed from the filename prefix.
	Number int
	// Width is the zero-padding width used in the filename.
	Width int
	// Slug is the filename between the number and ".md".
	Slug string
	// Title is the first "# " heading, falling back to the slug.
	Title string
	// Type is frontmatter "type", empty when absent.
	Type string
	// Status is frontmatter "status", empty when absent.
	Status string
	// Frontmatter holds every parsed frontmatter key.
	Frontmatter map[string]any
	// Body is the markdown after the frontmatter block.
	Body string
	// Created is the git commit date the file first appeared under any name;
	// zero when unknown.
	Created time.Time
	// Modified is the last git commit date touching the file; zero when unknown.
	Modified time.Time
	// Err records a parse problem (e.g. invalid YAML) without dropping the document.
	Err error
}

// DocType is one document type the convention documents.
type DocType struct {
	Name           string
	Description    string
	StatusRequired bool
}

// Types, KnownTypes and KnownStatuses are the vocabulary the convention
// documents. Unknown values are permitted and reported as lint warnings, never
// errors.
var (
	Types = []DocType{
		{"reference", "evergreen", false},
		{"review", "point-in-time assessment", true},
		{"task", "unit of work adding something new", true},
		{"bug", "unit of work fixing existing behaviour", true},
	}
	KnownTypes    = typeNames(Types)
	KnownStatuses = []string{"todo", "planned", "in-progress", "blocked", "open", "done", "superseded"}
)

// StatusTypes names the known types that require a status.
func StatusTypes() []string {
	var names []string
	for _, docType := range Types {
		if docType.StatusRequired {
			names = append(names, docType.Name)
		}
	}
	return names
}

// StatusRequired reports whether a known type requires a status; unknown types
// do not.
func StatusRequired(name string) bool {
	for _, docType := range Types {
		if docType.Name == name {
			return docType.StatusRequired
		}
	}
	return false
}

func typeNames(types []DocType) []string {
	names := make([]string, 0, len(types))
	for _, docType := range types {
		names = append(names, docType.Name)
	}
	return names
}
