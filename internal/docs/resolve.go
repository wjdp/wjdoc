package docs

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// DefaultWidth is the padding width used when a project has no numbered docs.
const DefaultWidth = 3

// Resolve finds the single document a reference names. A reference is an
// integer, a padded number, a path or filename, or a fragment of the slug or
// title. Exact matches win over fragments; an ambiguous fragment is an error.
func Resolve(documents []Document, ref string) (Document, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return Document{}, fmt.Errorf("no document matches %q", ref)
	}

	if number, err := strconv.Atoi(trimmed); err == nil {
		matches := filterFunc(documents, func(document Document) bool {
			return document.Number >= 0 && document.Number == number
		})
		if found, err := single(matches, ref); found || err != nil {
			return matches[0], err
		}
	}

	wanted := filepath.ToSlash(trimmed)
	matches := filterFunc(documents, func(document Document) bool {
		return document.Path == wanted || pathBase(document.Path) == wanted
	})
	if found, err := single(matches, ref); found || err != nil {
		return matches[0], err
	}

	needle := strings.ToLower(trimmed)
	matches = filterFunc(documents, func(document Document) bool {
		return strings.Contains(strings.ToLower(document.Slug), needle) ||
			strings.Contains(strings.ToLower(document.Title), needle)
	})
	if found, err := single(matches, ref); found || err != nil {
		return matches[0], err
	}

	return Document{}, fmt.Errorf("no document matches %q", ref)
}

// single reports whether matches holds exactly one document, and returns an
// ambiguity error listing the candidates when it holds more.
func single(matches []Document, ref string) (bool, error) {
	switch len(matches) {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		lines := make([]string, 0, len(matches))
		for _, document := range matches {
			lines = append(lines, document.Path)
		}
		return false, fmt.Errorf("%q is ambiguous, matches:\n%s", ref, strings.Join(lines, "\n"))
	}
}

// Criteria selects documents. An empty field means no filtering on it; values
// match case-insensitively.
type Criteria struct {
	Types      []string
	Statuses   []string
	Priorities []string
	// Tags must all be present. A tag ending in ":" matches any tag in that
	// namespace.
	Tags []string
}

// Filter keeps the documents that meet every criterion.
func Filter(documents []Document, criteria Criteria) []Document {
	return filterFunc(documents, func(document Document) bool {
		return matchesAny(document.Type, criteria.Types) &&
			matchesAny(document.Status, criteria.Statuses) &&
			matchesAny(document.Priority, criteria.Priorities) &&
			hasAllTags(document.Tags, criteria.Tags)
	})
}

func hasAllTags(tags, wanted []string) bool {
	for _, want := range wanted {
		if !hasTag(tags, strings.ToLower(strings.TrimSpace(want))) {
			return false
		}
	}
	return true
}

func hasTag(tags []string, want string) bool {
	for _, tag := range tags {
		if tag == want || strings.HasSuffix(want, ":") && strings.HasPrefix(tag, want) {
			return true
		}
	}
	return false
}

func matchesAny(value string, wanted []string) bool {
	if len(wanted) == 0 {
		return true
	}
	for _, candidate := range wanted {
		if strings.EqualFold(strings.TrimSpace(candidate), value) {
			return true
		}
	}
	return false
}

// Width is the most common zero-padding width among numbered documents,
// falling back to DefaultWidth when there are none. Ties favour the wider.
func Width(documents []Document) int {
	counts := map[int]int{}
	for _, document := range documents {
		if document.Number >= 0 && document.Width > 0 {
			counts[document.Width]++
		}
	}

	best, bestCount := DefaultWidth, 0
	for width, count := range counts {
		if count > bestCount || (count == bestCount && width > best) {
			best, bestCount = width, count
		}
	}
	return best
}

func filterFunc(documents []Document, keep func(Document) bool) []Document {
	kept := make([]Document, 0, len(documents))
	for _, document := range documents {
		if keep(document) {
			kept = append(kept, document)
		}
	}
	return kept
}

func pathBase(path string) string {
	if index := strings.LastIndex(path, "/"); index >= 0 {
		return path[index+1:]
	}
	return path
}
