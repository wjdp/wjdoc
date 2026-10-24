package render

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/wjdp/wjdoc/internal/docs"
)

// Show writes a single document in the given mode. bodyOnly strips
// frontmatter in Pretty and Plain; JSON always carries the parsed body
// separately and ignores bodyOnly.
func Show(w io.Writer, doc docs.Document, mode Mode, bodyOnly bool) {
	switch mode {
	case JSON:
		showJSON(w, doc)
	case Plain:
		showPlain(w, doc, bodyOnly)
	default:
		showPretty(w, doc, bodyOnly)
	}
}

func showJSON(w io.Writer, doc docs.Document) {
	out := jsonShowDoc{jsonDoc: toJSONDoc(doc), Body: doc.Body}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func showPlain(w io.Writer, doc docs.Document, bodyOnly bool) {
	fmt.Fprint(w, content(doc, bodyOnly))
}

func showPretty(w io.Writer, doc docs.Document, bodyOnly bool) {
	if !bodyOnly {
		parts := []string{doc.Path, displayOrDash(doc.Type), displayOrDash(doc.Status)}
		if doc.Priority != "" {
			parts = append(parts, doc.Priority)
		}
		if len(doc.Tags) > 0 {
			parts = append(parts, strings.Join(doc.Tags, ", "))
		}
		fmt.Fprintln(w, headerStyle.Render(strings.Join(parts, " · ")))
	}
	fmt.Fprint(w, content(doc, bodyOnly))
}

// content returns the doc's body when bodyOnly, otherwise the file's raw
// contents (frontmatter fence included, since it is not retained on
// Document).
func content(doc docs.Document, bodyOnly bool) string {
	if bodyOnly {
		return doc.Body
	}
	raw, err := os.ReadFile(doc.AbsPath)
	if err != nil {
		return doc.Body
	}
	return string(raw)
}
