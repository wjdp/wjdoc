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
	// Modified is the last git commit date touching the file; zero when unknown.
	Modified time.Time
	// Err records a parse problem (e.g. invalid YAML) without dropping the document.
	Err error
}

// KnownTypes and KnownStatuses are the vocabulary the convention documents.
// Unknown values are permitted and reported as lint warnings, never errors.
var (
	KnownTypes    = []string{"reference", "review", "task"}
	KnownStatuses = []string{"todo", "planned", "in-progress", "blocked", "open", "done", "superseded"}
)
