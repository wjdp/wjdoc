package render

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wjdp/wjdoc/internal/docs"
)

func sampleDocs() []docs.Document {
	return []docs.Document{
		{
			Path: "docs/001-Sync-robustness.md", AbsPath: "/repo/docs/001-Sync-robustness.md",
			Number: 1, Type: "task", Status: "in-progress", Title: "Sync robustness",
			Modified: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		},
		{
			Path: "docs/002-No-frontmatter.md", AbsPath: "/repo/docs/002-No-frontmatter.md",
			Number: 2, Title: "No frontmatter",
			Err: errors.New("missing type"),
		},
	}
}

func TestListPlain(t *testing.T) {
	var buf strings.Builder
	List(&buf, sampleDocs(), Plain, 0)

	want := "docs/001-Sync-robustness.md\ttask\tin-progress\tSync robustness\n" +
		"docs/002-No-frontmatter.md\t\t\tNo frontmatter\n"
	if buf.String() != want {
		t.Errorf("got:\n%q\nwant:\n%q", buf.String(), want)
	}
}

func TestListJSON(t *testing.T) {
	var buf strings.Builder
	List(&buf, sampleDocs(), JSON, 0)

	want := `[
  {
    "number": 1,
    "path": "docs/001-Sync-robustness.md",
    "type": "task",
    "status": "in-progress",
    "title": "Sync robustness",
    "modified": "2026-01-02T03:04:05Z",
    "error": null
  },
  {
    "number": 2,
    "path": "docs/002-No-frontmatter.md",
    "type": "",
    "status": "",
    "title": "No frontmatter",
    "modified": null,
    "error": "missing type"
  }
]
`
	if buf.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), want)
	}
}

func TestListPrettyContainsContent(t *testing.T) {
	for _, width := range []int{80, 160} {
		var buf strings.Builder
		List(&buf, sampleDocs(), Pretty, width)

		out := buf.String()
		if !strings.Contains(out, "docs/001-Sync-robustness.md") {
			t.Errorf("width %d: missing path in output:\n%s", width, out)
		}
		if width >= 120 && !strings.Contains(out, "Sync robustness") {
			t.Errorf("width %d: missing title in output:\n%s", width, out)
		}
	}
}
