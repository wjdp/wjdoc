package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var namePattern = regexp.MustCompile(`^(\d+)-(.+)\.md$`)

// ParseName splits a filename of the form "007-Sync-robustness.md" into its
// number, zero-padding width and slug.
func ParseName(name string) (number, width int, slug string, ok bool) {
	match := namePattern.FindStringSubmatch(name)
	if match == nil {
		return -1, 0, strings.TrimSuffix(name, ".md"), false
	}
	number, err := strconv.Atoi(match[1])
	if err != nil {
		return -1, 0, strings.TrimSuffix(name, ".md"), false
	}
	return number, len(match[1]), match[2], true
}

// Load reads every markdown file directly inside root/docs and inside its
// ref/ and task/ folders, sorted by filename so that numbers interleave across
// folders. Files that do not match the naming convention are still returned,
// with Number -1, so that lint can report them.
func Load(root string) ([]Document, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	docsDir := filepath.Join(root, "docs")
	paths, err := markdownIn(docsDir)
	if err != nil {
		return nil, err
	}
	for _, folder := range Folders {
		inFolder, err := markdownIn(filepath.Join(docsDir, folder))
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		paths = append(paths, inFolder...)
	}
	sort.Slice(paths, func(i, j int) bool {
		left, right := strings.ToLower(filepath.Base(paths[i])), strings.ToLower(filepath.Base(paths[j]))
		if left != right {
			return left < right
		}
		if filepath.Base(paths[i]) != filepath.Base(paths[j]) {
			return filepath.Base(paths[i]) < filepath.Base(paths[j])
		}
		return paths[i] < paths[j]
	})

	documents := make([]Document, 0, len(paths))
	for _, absPath := range paths {
		source, err := os.ReadFile(absPath)
		if err != nil {
			return nil, err
		}
		documents = append(documents, newDocument(root, absPath, filepath.Base(absPath), string(source)))
	}
	return documents, nil
}

// markdownIn lists the absolute paths of the markdown files directly inside
// directory.
func markdownIn(directory string) ([]string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && isMarkdown(entry.Name()) {
			paths = append(paths, filepath.Join(directory, entry.Name()))
		}
	}
	return paths, nil
}

func newDocument(root, absPath, name, source string) Document {
	number, width, slug, _ := ParseName(name)

	relative, err := filepath.Rel(root, absPath)
	if err != nil {
		relative = filepath.Join("docs", name)
	}

	document := Document{
		Path:        filepath.ToSlash(relative),
		AbsPath:     absPath,
		Number:      number,
		Width:       width,
		Slug:        slug,
		Frontmatter: map[string]any{},
	}

	frontmatter, body := splitFrontmatter(source)
	document.Body = body
	if frontmatter != "" {
		mapping, err := decodeFrontmatter([]byte(frontmatter))
		if err != nil {
			document.Err = fmt.Errorf("%s: %w", document.Path, err)
		} else {
			document.Frontmatter = mapping
			document.Type = stringField(mapping, "type")
			document.Status = stringField(mapping, "status")
			document.Priority = stringField(mapping, "priority")
			document.Tags = tagsField(mapping)
		}
	}

	document.Title = heading(body)
	if document.Title == "" {
		document.Title = strings.ReplaceAll(slug, "-", " ")
	}
	return document
}

// splitFrontmatter returns the YAML between a leading "---" fence and the next
// "---" line, plus the remaining body. Without a fence the whole source is body.
func splitFrontmatter(source string) (frontmatter, body string) {
	rest, ok := trimFirstLine(source, "---")
	if !ok {
		return "", source
	}
	lines := strings.Split(rest, "\n")
	for index, line := range lines {
		if strings.TrimRight(line, "\r") == "---" {
			return strings.Join(lines[:index], "\n"), strings.Join(lines[index+1:], "\n")
		}
	}
	return "", source
}

func trimFirstLine(source, want string) (string, bool) {
	line, rest, found := strings.Cut(source, "\n")
	if strings.TrimRight(line, "\r") != want {
		return source, false
	}
	if !found {
		return "", true
	}
	return rest, true
}

func heading(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

func stringField(mapping map[string]any, key string) string {
	value, present := mapping[key]
	if !present || value == nil {
		return ""
	}
	text, isString := value.(string)
	if !isString {
		text = fmt.Sprint(value)
	}
	return strings.ToLower(strings.TrimSpace(text))
}

// tagsField is the "tags" list, lowercased and trimmed, or nil when the key is
// absent or holds anything but a list of strings; lint reports the latter.
func tagsField(mapping map[string]any) []string {
	items, isList := mapping["tags"].([]any)
	if !isList {
		return nil
	}
	tags := make([]string, 0, len(items))
	for _, item := range items {
		text, isString := item.(string)
		if !isString {
			return nil
		}
		tags = append(tags, strings.ToLower(strings.TrimSpace(text)))
	}
	return tags
}
