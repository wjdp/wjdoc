package docs

import (
	"strings"

	"github.com/goccy/go-yaml"
)

// SetField edits one key in the frontmatter block, textually, leaving every
// other byte of the file alone: other keys, their order, comments, line endings
// and the body all survive. A missing key is appended to the block; a file
// without frontmatter gains a block at the top.
func SetField(source []byte, key, value string) ([]byte, error) {
	quoted, err := scalar(value)
	if err != nil {
		return nil, err
	}
	return setEntry(source, key, key+": "+quoted), nil
}

// SetList writes a key as a one-line flow sequence, replacing any block-style
// list it had, with the same guarantees as SetField. An empty list removes the
// key.
func SetList(source []byte, key string, values []string) ([]byte, error) {
	if len(values) == 0 {
		return UnsetField(source, key)
	}
	items := make([]string, 0, len(values))
	for _, value := range values {
		item, err := flowScalar(value)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return setEntry(source, key, key+": ["+strings.Join(items, ", ")+"]"), nil
}

func setEntry(source []byte, key, entry string) []byte {
	b := findBlock(source)
	if !b.found {
		return []byte("---\n" + entry + "\n---\n\n" + string(source))
	}

	lines := b.lines()
	if start, end := spanOfKey(lines, key); start >= 0 {
		edited := append(lines[:start:start], entry+carriageReturn(lines[start]))
		return b.replace(append(edited, lines[end:]...))
	}
	return b.replace(append(lines, entry+b.lineSuffix()))
}

// UnsetField removes a key from the frontmatter block, leaving the rest of the
// file byte for byte. Removing an absent key is not an error.
func UnsetField(source []byte, key string) ([]byte, error) {
	b := findBlock(source)
	if !b.found {
		return source, nil
	}

	lines := b.lines()
	start, end := spanOfKey(lines, key)
	if start < 0 {
		return source, nil
	}
	return b.replace(append(lines[:start:start], lines[end:]...)), nil
}

// scalar renders a value as YAML would write it, quoting only when a bare
// string would be misread.
func scalar(value string) (string, error) {
	encoded, err := yaml.Marshal(value)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(encoded), "\n"), nil
}

// flowScalar renders a value for a flow sequence: bare when it has the shape
// of a tag, otherwise quoted so commas and brackets cannot split it.
func flowScalar(value string) (string, error) {
	if TagPattern.MatchString(value) {
		return value, nil
	}
	encoded, err := yaml.MarshalWithOptions(value, yaml.JSON())
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(encoded), "\n"), nil
}

// block locates the frontmatter between the leading fence and its closing
// fence, as byte offsets into the source.
type block struct {
	source      []byte
	found       bool
	start, end  int
	lineEndings string
}

func findBlock(source []byte) block {
	text := string(source)
	rest, ok := strings.CutPrefix(text, "---\n")
	openLength := len("---\n")
	if !ok {
		if rest, ok = strings.CutPrefix(text, "---\r\n"); ok {
			openLength = len("---\r\n")
		}
	}
	if !ok {
		return block{source: source}
	}

	endings := "\n"
	if openLength == len("---\r\n") {
		endings = "\r\n"
	}

	offset := openLength
	for _, line := range strings.SplitAfter(rest, "\n") {
		if strings.TrimRight(strings.TrimSuffix(line, "\n"), "\r") == "---" {
			return block{source: source, found: true, start: openLength, end: offset, lineEndings: endings}
		}
		offset += len(line)
	}
	return block{source: source}
}

func (b block) lines() []string {
	inner := string(b.source[b.start:b.end])
	inner = strings.TrimSuffix(inner, "\n")
	if inner == "" {
		return nil
	}
	return strings.Split(inner, "\n")
}

func (b block) replace(lines []string) []byte {
	inner := ""
	if len(lines) > 0 {
		inner = strings.Join(lines, "\n") + "\n"
	}
	return []byte(string(b.source[:b.start]) + inner + string(b.source[b.end:]))
}

// lineSuffix is the carriage return a CRLF file needs on a line appended to
// the block.
func (b block) lineSuffix() string {
	if b.lineEndings == "\r\n" {
		return "\r"
	}
	return ""
}

func carriageReturn(line string) string {
	if strings.HasSuffix(line, "\r") {
		return "\r"
	}
	return ""
}

// spanOfKey is the half-open range of lines a top-level key occupies: its own
// line plus any indented or "- " continuation lines, such as a block list.
// start is -1 when the key is absent.
func spanOfKey(lines []string, key string) (start, end int) {
	for index, line := range lines {
		name, _, found := strings.Cut(line, ":")
		if !found || name != key {
			continue
		}
		end = index + 1
		for end < len(lines) && continuesEntry(lines[end]) {
			end++
		}
		return index, end
	}
	return -1, -1
}

func continuesEntry(line string) bool {
	return strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "-")
}
