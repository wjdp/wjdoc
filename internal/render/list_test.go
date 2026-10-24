package render

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/wjdp/wjdoc/internal/docs"
)

// TestMain forces a colour profile so that the styling assertions below see
// the escape sequences a terminal would.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	os.Exit(m.Run())
}

func sampleDocs() []docs.Document {
	return []docs.Document{
		{
			Path: "docs/001-Sync-robustness.md", AbsPath: "/repo/docs/001-Sync-robustness.md",
			Number: 1, Width: 3, Type: "task", Status: "in-progress", Title: "Sync robustness",
			Priority: "p1", Tags: []string{"area:sync", "reach:app"},
			Created:  time.Date(2025, 11, 2, 3, 4, 5, 0, time.UTC),
			Modified: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		},
		{
			Path: "docs/002-No-frontmatter.md", AbsPath: "/repo/docs/002-No-frontmatter.md",
			Number: 2, Width: 3, Title: "No frontmatter",
			Err: errors.New("missing type"),
		},
	}
}

func tableDocs() []docs.Document {
	return []docs.Document{
		{
			Path: "docs/001-Sync-robustness.md", AbsPath: "/repo/docs/001-Sync-robustness.md",
			Number: 1, Width: 3, Type: "task", Status: "in-progress",
			Title:    "Sync robustness across flaky connections and long offline spells",
			Created:  time.Date(2025, 11, 2, 3, 4, 5, 0, time.UTC),
			Modified: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		},
		{
			Path: "docs/002-Loader.md", AbsPath: "/repo/docs/002-Loader.md",
			Number: 2, Width: 3, Type: "task", Status: "done", Title: "Ship the loader",
		},
		{
			Path: "docs/003-Old-plan.md", AbsPath: "/repo/docs/003-Old-plan.md",
			Number: 3, Width: 3, Type: "task", Status: "superseded", Title: "Old plan",
		},
		{
			Path: "docs/000-Docs.md", AbsPath: "/repo/docs/000-Docs.md",
			Number: 0, Width: 3, Type: "reference", Title: "Docs",
		},
		{
			Path: "docs/README.md", AbsPath: "/repo/docs/README.md",
			Number: -1, Title: "Readme",
		},
	}
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m|\x1b\]8;;[^\x1b]*\x1b\\`)

func plainText(out string) string {
	return ansiPattern.ReplaceAllString(out, "")
}

func pretty(t *testing.T, list []docs.Document, width int, showPaths bool) string {
	t.Helper()
	var buf strings.Builder
	List(&buf, list, Pretty, width, Columns{Paths: showPaths})
	return buf.String()
}

func TestListPlainHasEightFields(t *testing.T) {
	var buf strings.Builder
	List(&buf, sampleDocs(), Plain, 0, Columns{})

	want := "docs/001-Sync-robustness.md\ttask\tin-progress\tSync robustness\t2025-11-02\t2026-01-02\tp1\tarea:sync,reach:app\n" +
		"docs/002-No-frontmatter.md\t\t\tNo frontmatter\t\t\t\t\n"
	if buf.String() != want {
		t.Errorf("got:\n%q\nwant:\n%q", buf.String(), want)
	}
	for _, line := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
		if fields := strings.Split(line, "\t"); len(fields) != 8 {
			t.Errorf("line %q has %d fields, want 8", line, len(fields))
		}
	}
}

func TestListJSON(t *testing.T) {
	var buf strings.Builder
	List(&buf, sampleDocs(), JSON, 0, Columns{})

	want := `[
  {
    "number": 1,
    "path": "docs/001-Sync-robustness.md",
    "type": "task",
    "status": "in-progress",
    "priority": "p1",
    "tags": [
      "area:sync",
      "reach:app"
    ],
    "title": "Sync robustness",
    "created": "2025-11-02T03:04:05Z",
    "modified": "2026-01-02T03:04:05Z",
    "error": null
  },
  {
    "number": 2,
    "path": "docs/002-No-frontmatter.md",
    "type": "",
    "status": "",
    "priority": "",
    "tags": [],
    "title": "No frontmatter",
    "created": null,
    "modified": null,
    "error": "missing type"
  }
]
`
	if buf.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), want)
	}
}

func TestListPrettyDropsColumnsAsTheTerminalNarrows(t *testing.T) {
	cases := []struct {
		width   int
		present []string
		absent  []string
	}{
		{120, []string{"Type", "Created", "Modified"}, nil},
		{80, []string{"Type", "Created", "Modified"}, nil},
		{70, []string{"Created", "Modified"}, []string{"Type"}},
		{60, nil, []string{"Type", "Created", "Modified"}},
	}

	for _, c := range cases {
		header := strings.Split(plainText(pretty(t, tableDocs(), c.width, false)), "\n")[0]
		for _, want := range c.present {
			if !strings.Contains(header, want) {
				t.Errorf("width %d: header %q has no %q column", c.width, header, want)
			}
		}
		for _, unwanted := range c.absent {
			if strings.Contains(header, unwanted) {
				t.Errorf("width %d: header %q still has the %q column", c.width, header, unwanted)
			}
		}
	}
}

func TestListPrettyFitsTheTerminalWidth(t *testing.T) {
	for _, width := range []int{60, 70, 80, 120} {
		for _, line := range strings.Split(strings.TrimRight(plainText(pretty(t, tableDocs(), width, false)), "\n"), "\n") {
			if len([]rune(line)) > width {
				t.Errorf("width %d: line is %d cells:\n%s", width, len([]rune(line)), line)
			}
		}
	}
}

func TestListPrettyTruncatesTheTitle(t *testing.T) {
	out := plainText(pretty(t, tableDocs(), 60, false))

	if !strings.Contains(out, "…") {
		t.Errorf("long title was not truncated:\n%s", out)
	}
}

func TestListPrettyNumbersAndLinks(t *testing.T) {
	out := pretty(t, tableDocs(), 120, false)

	if !strings.Contains(out, "file:///repo/docs/001-Sync-robustness.md") {
		t.Errorf("no hyperlink to the document:\n%s", out)
	}
	if !strings.Contains(plainText(out), "001") {
		t.Errorf("no padded number:\n%s", plainText(out))
	}
	if !strings.Contains(plainText(out), "README.md") {
		t.Errorf("an unnumbered file does not show its filename:\n%s", plainText(out))
	}
}

func TestListPrettyFadesFinishedRowsWithoutHidingThem(t *testing.T) {
	out := pretty(t, tableDocs(), 120, true)

	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(plainText(line), "docs/002-Loader.md") {
			continue
		}
		if !strings.Contains(line, "\x1b[2m") {
			t.Errorf("done row is not faint: %q", line)
		}
		if !strings.Contains(plainText(line), "002") {
			t.Errorf("done row lost its number: %q", plainText(line))
		}
		return
	}
	t.Errorf("no row for the done document:\n%s", plainText(out))
}

func TestListPrettyStrikesThroughSupersededTitles(t *testing.T) {
	out := pretty(t, tableDocs(), 120, false)

	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(plainText(line), "Old plan") && strings.Contains(line, ";9m") {
			return
		}
	}
	t.Errorf("superseded title is not struck through:\n%s", out)
}

func TestListPrettyPathsColumnSurvivesNarrowTerminals(t *testing.T) {
	for _, width := range []int{60, 120} {
		out := plainText(pretty(t, tableDocs(), width, true))
		if !strings.Contains(out, "Document") || !strings.Contains(out, "docs/001-Sync-robustness.md") {
			t.Errorf("width %d: --paths column missing:\n%s", width, out)
		}
	}
}

func TestListPrettyFooterCountsByStatus(t *testing.T) {
	out := plainText(pretty(t, tableDocs(), 120, false))

	want := "1 in-progress · 1 done · 1 superseded · 2 reference"
	if !strings.Contains(out, want) {
		t.Errorf("footer %q missing from:\n%s", want, out)
	}
}

func TestListPrettyErrorRowShowsTheMessage(t *testing.T) {
	out := plainText(pretty(t, sampleDocs(), 120, false))

	if !strings.Contains(out, "missing type") {
		t.Errorf("no error message in the table:\n%s", out)
	}
	if !strings.Contains(out, "invalid frontmatter docs/002-No-frontmatter.md") {
		t.Errorf("no error line after the footer:\n%s", out)
	}
}

func TestRelativeAge(t *testing.T) {
	now := time.Now()
	cases := []struct {
		modified time.Time
		want     string
	}{
		{time.Time{}, ""},
		{now, "today"},
		{now.AddDate(0, 0, -3), "3d"},
		{now.AddDate(0, 0, -14), "2w"},
		{now.AddDate(0, 0, -150), "5mo"},
		{now.AddDate(-1, 0, 0), "1y"},
	}
	for _, c := range cases {
		if got := RelativeAge(c.modified); got != c.want {
			t.Errorf("RelativeAge(%v) = %q, want %q", c.modified, got, c.want)
		}
	}
}

func TestListPrettyShowsPriorityOnlyWhenPresent(t *testing.T) {
	if header := strings.SplitN(plainText(pretty(t, tableDocs(), 120, false)), "\n", 2)[0]; strings.Contains(header, "Pri") {
		t.Errorf("Pri column shown with no priorities: %q", header)
	}

	list := tableDocs()
	list[0].Priority = "p1"
	if header := strings.SplitN(plainText(pretty(t, list, 120, false)), "\n", 2)[0]; !strings.Contains(header, "Pri") {
		t.Errorf("Pri column missing: %q", header)
	}
}

func TestListPrettyAddsTagsOnRequest(t *testing.T) {
	list := tableDocs()
	list[1].Tags = []string{"area:engine"}

	var buf strings.Builder
	List(&buf, list, Pretty, 160, Columns{Tags: true})
	out := plainText(buf.String())
	if !strings.Contains(out, "Tags") || !strings.Contains(out, "area:engine") {
		t.Errorf("tags column missing:\n%s", out)
	}
}

func TestListPrettyColoursTypes(t *testing.T) {
	out := pretty(t, tableDocs(), 120, false)

	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(plainText(line), "Docs") {
			if !strings.Contains(line, typeStyles["reference"].Render("reference")) {
				t.Errorf("reference type is not coloured: %q", line)
			}
			return
		}
	}
	t.Errorf("no row for the reference:\n%s", plainText(out))
}
