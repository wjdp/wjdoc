package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlug(t *testing.T) {
	cases := []struct{ title, want string }{
		{"prisma to Drizzle", "Prisma-to-Drizzle"},
		{"  Sync   robustness  ", "Sync-robustness"},
		{"Ship the loader!", "Ship-the-loader"},
		{"wj: design", "Wj-design"},
		{"already-hyphenated", "Already-hyphenated"},
		{"HTTP/2 support", "HTTP2-support"},
		{"-- leading and trailing --", "Leading-and-trailing"},
		{"!!!", ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.title, func(t *testing.T) {
			if got := Slug(testCase.title); got != testCase.want {
				t.Errorf("Slug(%q) = %q, want %q", testCase.title, got, testCase.want)
			}
		})
	}
}

func TestNextNumber(t *testing.T) {
	documents := []Document{{Number: -1}, {Number: 3}, {Number: 1}}

	cases := []struct {
		name      string
		documents []Document
		extra     []string
		want      int
	}{
		{"documents only", documents, nil, 4},
		{"extra names win", documents, []string{"09-Elsewhere.md", "README.md"}, 10},
		{"nothing numbered", []Document{{Number: -1}}, nil, 0},
		{"empty", nil, nil, 0},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := NextNumber(testCase.documents, testCase.extra); got != testCase.want {
				t.Errorf("got %d, want %d", got, testCase.want)
			}
		})
	}
}

func TestPad(t *testing.T) {
	if got, err := Pad(7, 3); err != nil || got != "007" {
		t.Errorf("Pad(7, 3) = %q, %v", got, err)
	}
	if got, err := Pad(100, 3); err != nil || got != "100" {
		t.Errorf("Pad(100, 3) = %q, %v", got, err)
	}

	_, err := Pad(100, 2)
	if err == nil {
		t.Fatal("Pad(100, 2) did not fail")
	}
	if !strings.Contains(err.Error(), "wj repad --width 3") {
		t.Errorf("error does not point at repad: %v", err)
	}
}

func TestCreateWritesFrontmatterAndHeading(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		docType  string
		title    string
		wantPath string
		want     string
	}{
		{"task", "Try it out", "docs/07-Try-it-out.md", "---\ntype: task\nstatus: todo\n---\n\n# Try it out\n"},
		{"review", "Loader review", "docs/08-Loader-review.md", "---\ntype: review\nstatus: open\n---\n\n# Loader review\n"},
		{"reference", "About these docs", "docs/09-About-these-docs.md", "---\ntype: reference\n---\n\n# About these docs\n"},
		{"note", "Unknown type", "docs/10-Unknown-type.md", "---\ntype: note\nstatus: todo\n---\n\n# Unknown type\n"},
		{"bug", "Fix the loader", "docs/11-Fix-the-loader.md", "---\ntype: bug\nstatus: todo\n---\n\n# Fix the loader\n"},
	}

	for index, testCase := range cases {
		path, err := Create(root, 7+index, 2, testCase.docType, testCase.title)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if path != testCase.wantPath {
			t.Errorf("got path %q, want %q", path, testCase.wantPath)
		}

		source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if string(source) != testCase.want {
			t.Errorf("%s contents:\n%q\nwant:\n%q", path, source, testCase.want)
		}
	}
}

func TestCreateRefusesToOverwrite(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := Create(root, 1, 3, "task", "Once"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := Create(root, 1, 3, "task", "Once"); err == nil {
		t.Fatal("second Create did not fail")
	}
}

func TestCreateRefusesAnUnpaddableNumber(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := Create(root, 100, 2, "task", "Too big"); err == nil {
		t.Fatal("Create did not refuse an overflowing number")
	}
}
