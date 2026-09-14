package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wjdp/wjdoc/internal/docs"
)

func sampleShowDoc(t *testing.T) docs.Document {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "001-Example.md")
	raw := "---\ntype: task\nstatus: todo\n---\n\n# Example\n\nBody text.\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	return docs.Document{
		Path: "docs/001-Example.md", AbsPath: path,
		Number: 1, Type: "task", Status: "todo", Title: "Example",
		Body: "\n# Example\n\nBody text.\n",
	}
}

func TestShowPlainFullFile(t *testing.T) {
	var buf strings.Builder
	Show(&buf, sampleShowDoc(t), Plain, false)

	want := "---\ntype: task\nstatus: todo\n---\n\n# Example\n\nBody text.\n"
	if buf.String() != want {
		t.Errorf("got:\n%q\nwant:\n%q", buf.String(), want)
	}
}

func TestShowPlainBodyOnly(t *testing.T) {
	doc := sampleShowDoc(t)
	var buf strings.Builder
	Show(&buf, doc, Plain, true)

	if buf.String() != doc.Body {
		t.Errorf("got:\n%q\nwant:\n%q", buf.String(), doc.Body)
	}
}

func TestShowJSON(t *testing.T) {
	doc := sampleShowDoc(t)
	var buf strings.Builder
	Show(&buf, doc, JSON, true)

	want := `{
  "number": 1,
  "path": "docs/001-Example.md",
  "type": "task",
  "status": "todo",
  "title": "Example",
  "modified": null,
  "error": null,
  "body": "\n# Example\n\nBody text.\n"
}
`
	if buf.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), want)
	}
}

func TestShowPrettyHeaderOmittedOnBodyOnly(t *testing.T) {
	doc := sampleShowDoc(t)

	var withHeader strings.Builder
	Show(&withHeader, doc, Pretty, false)
	if !strings.Contains(withHeader.String(), doc.Path) {
		t.Errorf("expected header with path, got:\n%s", withHeader.String())
	}

	var bodyOnly strings.Builder
	Show(&bodyOnly, doc, Pretty, true)
	if strings.Contains(bodyOnly.String(), doc.Path) {
		t.Errorf("bodyOnly output should not contain header path, got:\n%s", bodyOnly.String())
	}
}
