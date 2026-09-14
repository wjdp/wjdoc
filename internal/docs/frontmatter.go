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
	entry := key + ": " + quoted

	b := findBlock(source)
	if !b.found {
		return []byte("---\n" + entry + "\n---\n\n" + string(source)), nil
	}

	lines := b.lines()
	if index := indexOfKey(lines, key); index >= 0 {
		lines[index] = entry + carriageReturn(lines[index])
		return b.replace(lines), nil
	}
	return b.replace(append(lines, entry+b.lineSuffix())), nil
}

// UnsetField removes a key from the frontmatter block, leaving the rest of the
// file byte for byte. Removing an absent key is not an error.
func UnsetField(source []byte, key string) ([]byte, error) {
	b := findBlock(source)
	if !b.found {
		return source, nil
	}

	lines := b.lines()
	index := indexOfKey(lines, key)
	if index < 0 {
		return source, nil
	}
	return b.replace(append(lines[:index:index], lines[index+1:]...)), nil
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

func indexOfKey(lines []string, key string) int {
	for index, line := range lines {
		name, _, found := strings.Cut(line, ":")
		if found && strings.TrimSpace(name) == key {
			return index
		}
	}
	return -1
}
