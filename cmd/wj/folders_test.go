package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	folderedFixture = "../../internal/docs/testdata/foldered"
	mixedFixture    = "../../internal/docs/testdata/mixed"
)

// copyTree copies a whole fixture project into a temporary directory so that
// writing commands never touch testdata.
func copyTree(t *testing.T, fixture string) string {
	t.Helper()

	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(fixture)); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestNewWritesIntoTheFolderThatSuitsTheType(t *testing.T) {
	cases := []struct{ fixture, wantReference, wantBug string }{
		{folderedFixture, "docs/ref/004-Glossary.md", "docs/task/005-Loader-drops-files.md"},
		{mixedFixture, "docs/ref/005-Glossary.md", "docs/task/006-Loader-drops-files.md"},
	}
	for _, testCase := range cases {
		root := copyTree(t, testCase.fixture)

		reference := strings.TrimSpace(runInOK(t, root, "new", "reference", "Glossary", "--plain"))
		bug := strings.TrimSpace(runInOK(t, root, "new", "bug", "Loader drops files", "--plain"))

		if reference != testCase.wantReference {
			t.Errorf("%s: reference written to %q, want %q", testCase.fixture, reference, testCase.wantReference)
		}
		if bug != testCase.wantBug {
			t.Errorf("%s: bug written to %q, want %q", testCase.fixture, bug, testCase.wantBug)
		}
	}
}

func TestNewCreatesTheMissingFolder(t *testing.T) {
	root := copyTree(t, folderedFixture)
	if err := os.RemoveAll(filepath.Join(root, "docs", "ref")); err != nil {
		t.Fatal(err)
	}

	out := runInOK(t, root, "new", "reference", "Glossary", "--json")
	var created struct{ Path string }
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatalf("decoding %q: %v", out, err)
	}
	if created.Path != "docs/ref/004-Glossary.md" {
		t.Errorf("path %q, want docs/ref/004-Glossary.md", created.Path)
	}
}

func TestNewInAFlatProjectStaysAtTheTop(t *testing.T) {
	root := copyFixture(t)
	if out := runInOK(t, root, "new", "reference", "Glossary", "--plain"); !strings.HasPrefix(out, "docs/07-Glossary.md") {
		t.Errorf("got %q, want docs/07-Glossary.md", out)
	}
}
