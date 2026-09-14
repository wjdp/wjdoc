package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureRoot = "testdata/basic"

func loadFixture(t *testing.T) []Document {
	t.Helper()
	documents, err := Load(fixtureRoot)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return documents
}

func byPath(t *testing.T, documents []Document, path string) Document {
	t.Helper()
	for _, document := range documents {
		if document.Path == path {
			return document
		}
	}
	t.Fatalf("no document with path %q", path)
	return Document{}
}

func TestParseName(t *testing.T) {
	cases := []struct {
		name       string
		wantNumber int
		wantWidth  int
		wantSlug   string
		wantOK     bool
	}{
		{"007-Sync-robustness.md", 7, 3, "Sync-robustness", true},
		{"00-Docs.md", 0, 2, "Docs", true},
		{"01-Sync robustness.md", 1, 2, "Sync robustness", true},
		{"1-One.md", 1, 1, "One", true},
		{"README.md", -1, 0, "README", false},
		{"007-.md", -1, 0, "007-", false},
		{"notes.txt", -1, 0, "notes.txt", false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			number, width, slug, ok := ParseName(testCase.name)
			if number != testCase.wantNumber || width != testCase.wantWidth || slug != testCase.wantSlug || ok != testCase.wantOK {
				t.Errorf("got (%d, %d, %q, %t), want (%d, %d, %q, %t)",
					number, width, slug, ok,
					testCase.wantNumber, testCase.wantWidth, testCase.wantSlug, testCase.wantOK)
			}
		})
	}
}

func TestLoadOrderAndFields(t *testing.T) {
	documents := loadFixture(t)

	wantOrder := []string{
		"docs/00-Docs.md",
		"docs/01-Sync robustness.md",
		"docs/02-No-frontmatter.md",
		"docs/03-Invalid-yaml.md",
		"docs/04-Ship-the-loader.md",
		"docs/05-Duplicate-number.md",
		"docs/05-Loader-review.md",
		"docs/06-No-heading-here.md",
		"docs/README.md",
	}
	if len(documents) != len(wantOrder) {
		t.Fatalf("loaded %d documents, want %d", len(documents), len(wantOrder))
	}
	for index, want := range wantOrder {
		if documents[index].Path != want {
			t.Errorf("document %d is %q, want %q", index, documents[index].Path, want)
		}
	}

	cases := []struct {
		path       string
		wantNumber int
		wantWidth  int
		wantSlug   string
		wantTitle  string
		wantType   string
		wantStatus string
		wantErr    bool
	}{
		{"docs/00-Docs.md", 0, 2, "Docs", "About these docs", "reference", "", false},
		{"docs/01-Sync robustness.md", 1, 2, "Sync robustness", "Sync robustness", "task", "in-progress", false},
		{"docs/02-No-frontmatter.md", 2, 2, "No-frontmatter", "Plain file", "", "", false},
		{"docs/03-Invalid-yaml.md", 3, 2, "Invalid-yaml", "Invalid yaml", "", "", true},
		{"docs/04-Ship-the-loader.md", 4, 2, "Ship-the-loader", "Ship the loader", "task", "todo", false},
		{"docs/05-Duplicate-number.md", 5, 2, "Duplicate-number", "Duplicate number", "task", "done", false},
		{"docs/05-Loader-review.md", 5, 2, "Loader-review", "Loader review", "review", "open", false},
		{"docs/06-No-heading-here.md", 6, 2, "No-heading-here", "No heading here", "reference", "", false},
		{"docs/README.md", -1, 0, "README", "Readme", "", "", false},
	}

	for _, testCase := range cases {
		t.Run(testCase.path, func(t *testing.T) {
			document := byPath(t, documents, testCase.path)
			if document.Number != testCase.wantNumber {
				t.Errorf("Number = %d, want %d", document.Number, testCase.wantNumber)
			}
			if document.Width != testCase.wantWidth {
				t.Errorf("Width = %d, want %d", document.Width, testCase.wantWidth)
			}
			if document.Slug != testCase.wantSlug {
				t.Errorf("Slug = %q, want %q", document.Slug, testCase.wantSlug)
			}
			if document.Title != testCase.wantTitle {
				t.Errorf("Title = %q, want %q", document.Title, testCase.wantTitle)
			}
			if document.Type != testCase.wantType {
				t.Errorf("Type = %q, want %q", document.Type, testCase.wantType)
			}
			if document.Status != testCase.wantStatus {
				t.Errorf("Status = %q, want %q", document.Status, testCase.wantStatus)
			}
			if (document.Err != nil) != testCase.wantErr {
				t.Errorf("Err = %v, want error: %t", document.Err, testCase.wantErr)
			}
			if document.Frontmatter == nil {
				t.Error("Frontmatter is nil")
			}
			if !filepath.IsAbs(document.AbsPath) {
				t.Errorf("AbsPath %q is not absolute", document.AbsPath)
			}
			if strings.HasPrefix(document.Body, "---") {
				t.Errorf("Body retains the frontmatter fence: %q", document.Body)
			}
		})
	}
}

func TestLoadBodyExcludesFrontmatter(t *testing.T) {
	document := byPath(t, loadFixture(t), "docs/04-Ship-the-loader.md")
	if strings.Contains(document.Body, "status: todo") {
		t.Errorf("Body contains frontmatter: %q", document.Body)
	}
	if !strings.Contains(document.Body, "A todo task.") {
		t.Errorf("Body missing content: %q", document.Body)
	}
}

func TestLoadMissingDirectory(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatal("expected an error for a missing docs directory")
	}
}

func TestFindRoot(t *testing.T) {
	fixture, err := filepath.Abs(fixtureRoot)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		start string
	}{
		{"from the root itself", fixture},
		{"from docs", filepath.Join(fixture, "docs")},
		{"from a nested directory", filepath.Join(fixture, "nested", "deeper")},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root, err := FindRoot(testCase.start)
			if err != nil {
				t.Fatalf("FindRoot: %v", err)
			}
			if root != fixture {
				t.Errorf("root = %q, want %q", root, fixture)
			}
		})
	}
}

func TestFindRootFallsBackToGit(t *testing.T) {
	temporary := t.TempDir()
	nested := filepath.Join(temporary, "repo", "pkg", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(temporary, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := FindRoot(nested)
	if err != nil {
		t.Fatalf("FindRoot: %v", err)
	}
	if root != repo {
		t.Errorf("root = %q, want %q", root, repo)
	}
}

func TestFindRootPrefersDocsOverGit(t *testing.T) {
	temporary := t.TempDir()
	inner := filepath.Join(temporary, "outer", "inner")
	if err := os.MkdirAll(filepath.Join(inner, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(temporary, "outer", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := FindRoot(inner)
	if err != nil {
		t.Fatalf("FindRoot: %v", err)
	}
	if root != inner {
		t.Errorf("root = %q, want %q", root, inner)
	}
}

func TestResolve(t *testing.T) {
	documents := loadFixture(t)

	cases := []struct {
		name     string
		ref      string
		wantPath string
	}{
		{"integer", "4", "docs/04-Ship-the-loader.md"},
		{"padded number", "04", "docs/04-Ship-the-loader.md"},
		{"zero", "0", "docs/00-Docs.md"},
		{"path", "docs/04-Ship-the-loader.md", "docs/04-Ship-the-loader.md"},
		{"filename", "04-Ship-the-loader.md", "docs/04-Ship-the-loader.md"},
		{"filename with a space", "01-Sync robustness.md", "docs/01-Sync robustness.md"},
		{"non-conforming filename", "README.md", "docs/README.md"},
		{"slug fragment", "robust", "docs/01-Sync robustness.md"},
		{"fragment is case-insensitive", "ROBUST", "docs/01-Sync robustness.md"},
		{"title fragment", "heading", "docs/06-No-heading-here.md"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			document, err := Resolve(documents, testCase.ref)
			if err != nil {
				t.Fatalf("Resolve(%q): %v", testCase.ref, err)
			}
			if document.Path != testCase.wantPath {
				t.Errorf("path = %q, want %q", document.Path, testCase.wantPath)
			}
		})
	}
}

func TestResolvePrecedence(t *testing.T) {
	documents := loadFixture(t)

	t.Run("exact number beats fragment", func(t *testing.T) {
		document, err := Resolve(documents, "6")
		if err != nil {
			t.Fatal(err)
		}
		if document.Path != "docs/06-No-heading-here.md" {
			t.Errorf("path = %q", document.Path)
		}
	})

	t.Run("exact filename beats fragment", func(t *testing.T) {
		document, err := Resolve(documents, "05-Loader-review.md")
		if err != nil {
			t.Fatal(err)
		}
		if document.Path != "docs/05-Loader-review.md" {
			t.Errorf("path = %q", document.Path)
		}
	})
}

func TestResolveErrors(t *testing.T) {
	documents := loadFixture(t)

	cases := []struct {
		name          string
		ref           string
		wantContains  []string
		wantAmbiguous bool
	}{
		{
			name:          "duplicate number",
			ref:           "5",
			wantAmbiguous: true,
			wantContains:  []string{"docs/05-Duplicate-number.md", "docs/05-Loader-review.md"},
		},
		{
			name:          "fragment matching several",
			ref:           "loader",
			wantAmbiguous: true,
			wantContains:  []string{"docs/04-Ship-the-loader.md", "docs/05-Loader-review.md"},
		},
		{
			name:         "no match",
			ref:          "nothing-like-this",
			wantContains: []string{`no document matches "nothing-like-this"`},
		},
		{
			name:         "unused number",
			ref:          "99",
			wantContains: []string{`no document matches "99"`},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := Resolve(documents, testCase.ref)
			if err == nil {
				t.Fatalf("Resolve(%q) succeeded, want an error", testCase.ref)
			}
			if testCase.wantAmbiguous && !strings.Contains(err.Error(), "ambiguous") {
				t.Errorf("error %q does not report ambiguity", err)
			}
			for _, want := range testCase.wantContains {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
		})
	}
}

func TestFilter(t *testing.T) {
	documents := loadFixture(t)

	cases := []struct {
		name      string
		types     []string
		statuses  []string
		wantPaths []string
	}{
		{
			name:      "no filters keeps everything",
			wantPaths: nil,
		},
		{
			name:      "by type",
			types:     []string{"review"},
			wantPaths: []string{"docs/05-Loader-review.md"},
		},
		{
			name:      "type is case-insensitive",
			types:     []string{"ReVieW"},
			wantPaths: []string{"docs/05-Loader-review.md"},
		},
		{
			name:      "by status",
			statuses:  []string{"TODO", "Done"},
			wantPaths: []string{"docs/04-Ship-the-loader.md", "docs/05-Duplicate-number.md"},
		},
		{
			name:      "type and status together",
			types:     []string{"task"},
			statuses:  []string{"in-progress"},
			wantPaths: []string{"docs/01-Sync robustness.md"},
		},
		{
			name:      "unknown value matches nothing",
			types:     []string{"diagram"},
			wantPaths: []string{},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			filtered := Filter(documents, testCase.types, testCase.statuses)
			if testCase.wantPaths == nil {
				if len(filtered) != len(documents) {
					t.Fatalf("kept %d documents, want all %d", len(filtered), len(documents))
				}
				return
			}
			if len(filtered) != len(testCase.wantPaths) {
				t.Fatalf("kept %d documents, want %d", len(filtered), len(testCase.wantPaths))
			}
			for index, want := range testCase.wantPaths {
				if filtered[index].Path != want {
					t.Errorf("document %d is %q, want %q", index, filtered[index].Path, want)
				}
			}
		})
	}
}

func TestWidth(t *testing.T) {
	cases := []struct {
		name      string
		documents []Document
		want      int
	}{
		{"fixture", loadFixture(t), 2},
		{"no documents", nil, DefaultWidth},
		{"unnumbered only", []Document{{Number: -1}}, DefaultWidth},
		{
			name: "most common wins",
			documents: []Document{
				{Number: 1, Width: 2},
				{Number: 2, Width: 3},
				{Number: 3, Width: 3},
			},
			want: 3,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := Width(testCase.documents); got != testCase.want {
				t.Errorf("Width = %d, want %d", got, testCase.want)
			}
		})
	}
}
