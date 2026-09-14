package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// Slug turns a title into the filename slug: spaces become hyphens, the first
// letter is uppercased and the rest is left as written so proper nouns keep
// their capitals. Anything other than a letter, digit or hyphen is dropped.
func Slug(title string) string {
	var builder strings.Builder
	for _, field := range strings.Fields(title) {
		builder.WriteByte('-')
		for _, letter := range field {
			if unicode.IsLetter(letter) || unicode.IsDigit(letter) || letter == '-' {
				builder.WriteRune(letter)
			}
		}
	}

	slug := collapseHyphens(builder.String())
	if slug == "" {
		return ""
	}

	runes := []rune(slug)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func collapseHyphens(text string) string {
	var builder strings.Builder
	previousWasHyphen := false
	for _, letter := range text {
		if letter == '-' {
			previousWasHyphen = true
			continue
		}
		if previousWasHyphen && builder.Len() > 0 {
			builder.WriteByte('-')
		}
		previousWasHyphen = false
		builder.WriteRune(letter)
	}
	return builder.String()
}

// NextNumber is one past the highest number in use, counting both the loaded
// documents and extra filenames seen elsewhere, such as on other git branches.
// It is 0 when nothing is numbered yet.
func NextNumber(documents []Document, extraNames []string) int {
	highest := -1
	for _, document := range documents {
		if document.Number > highest {
			highest = document.Number
		}
	}
	for _, name := range extraNames {
		if number, _, _, ok := ParseName(name); ok && number > highest {
			highest = number
		}
	}
	return highest + 1
}

// Pad zero-pads a number to width digits, refusing a number that does not fit
// and pointing at the repad command that widens the whole directory.
func Pad(number, width int) (string, error) {
	padded := fmt.Sprintf("%0*d", width, number)
	if len(padded) > width {
		return "", fmt.Errorf("number %d does not fit in %d digits: run `wj repad --width %d` first", number, width, width+1)
	}
	return padded, nil
}

// Create writes a new document and returns its path relative to root. It
// refuses to overwrite an existing file.
func Create(root string, number, width int, docType, title string) (string, error) {
	padded, err := Pad(number, width)
	if err != nil {
		return "", err
	}

	title = strings.Join(strings.Fields(title), " ")
	slug := Slug(title)
	if slug == "" {
		return "", fmt.Errorf("title %q has no usable characters for a filename", title)
	}

	path := filepath.ToSlash(filepath.Join("docs", padded+"-"+slug+".md"))
	absPath := filepath.Join(root, filepath.FromSlash(path))

	file, err := os.OpenFile(absPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return "", fmt.Errorf("%s already exists", path)
		}
		return "", err
	}
	defer file.Close()

	if _, err := file.WriteString(newDocumentSource(docType, title)); err != nil {
		return "", err
	}
	return path, file.Close()
}

func newDocumentSource(docType, title string) string {
	var builder strings.Builder
	builder.WriteString("---\ntype: ")
	builder.WriteString(docType)
	builder.WriteByte('\n')
	if status := InitialStatus(docType); status != "" {
		builder.WriteString("status: ")
		builder.WriteString(status)
		builder.WriteByte('\n')
	}
	builder.WriteString("---\n\n# ")
	builder.WriteString(title)
	builder.WriteByte('\n')
	return builder.String()
}

// InitialStatus is the status a newly created document of this type starts in,
// empty for types the convention says carry no status.
func InitialStatus(docType string) string {
	switch strings.ToLower(strings.TrimSpace(docType)) {
	case "reference":
		return ""
	case "review":
		return "open"
	default:
		return "todo"
	}
}
