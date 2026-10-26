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

// snapshot maps every file under root to its contents.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()

	files := map[string]string{}
	err := filepath.WalkDir(root, func(absPath string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(root, absPath)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = read(t, absPath)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func sameFiles(t *testing.T, got, want map[string]string) {
	t.Helper()

	for name, content := range want {
		if got[name] != content {
			t.Errorf("%s differs:\n%s\nwant:\n%s", name, got[name], content)
		}
	}
	for name := range got {
		if _, present := want[name]; !present {
			t.Errorf("unexpected file %s", name)
		}
	}
}

func TestMoveAllThereAndBackRestoresEveryByte(t *testing.T) {
	root := copyTree(t, folderedFixture)
	original := snapshot(t, root)

	runInOK(t, root, "move", "--all", "--to", "top")
	flat := snapshot(t, root)
	if _, present := flat["docs/001-Design.md"]; !present {
		t.Fatalf("001 did not move to the top of docs/: %v", flat)
	}
	if got := flat["docs/001-Design.md"]; !strings.Contains(got, "[002](002-Ship-the-loader.md#plan)") || !strings.Contains(got, "[agent file](../AGENTS.md)") || !strings.Contains(got, "](images/diagram.svg)") {
		t.Errorf("links out of the moved 001 were not recomputed:\n%s", got)
	}
	if got := flat["docs/003-Loader-review.md"]; !strings.Contains(got, "[002](./002-Ship-the-loader.md)") {
		t.Errorf("the ./ form was not kept:\n%s", got)
	}
	if got := flat["src/main.go"]; !strings.Contains(got, "docs/001-Design.md and docs/002-Ship-the-loader.md") {
		t.Errorf("code references were not rewritten:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "ref")); !os.IsNotExist(err) {
		t.Errorf("the emptied docs/ref/ was left behind, so the project still reads as foldered")
	}
	if out := runInOK(t, root, "lint", "--plain"); out != "" {
		t.Errorf("lint after moving to the top:\n%s", out)
	}

	runInOK(t, root, "move", "--all")
	sameFiles(t, snapshot(t, root), original)
}

func TestMoveAllMigratesAMixedProject(t *testing.T) {
	root := copyTree(t, mixedFixture)

	out := runInOK(t, root, "move", "--all")
	if !strings.Contains(out, "moved 3 files") {
		t.Errorf("got %q, want three moves", out)
	}

	lint := runInOK(t, root, "lint", "--plain")
	for _, line := range lines(lint) {
		if !strings.Contains(line, "not read by wj") {
			t.Errorf("lint after migrating: %s", line)
		}
	}
	if got := read(t, filepath.Join(root, "src", "main.go")); !strings.Contains(got, "docs/task/004-Still-at-the-top.md") {
		t.Errorf("the flat code reference was not rewritten:\n%s", got)
	}
	if got := read(t, filepath.Join(root, "docs", "task", "004-Still-at-the-top.md")); !strings.Contains(got, "[the design](../ref/001-Design.md)") || !strings.Contains(got, "[agent file](../../AGENTS.md)") {
		t.Errorf("links out of the moved 004 were not recomputed:\n%s", got)
	}
}

func TestMoveDryRunChangesNothing(t *testing.T) {
	root := copyTree(t, mixedFixture)
	before := snapshot(t, root)

	out := runInOK(t, root, "move", "4", "--dry-run")
	if !strings.Contains(out, "docs/004-Still-at-the-top.md → docs/task/004-Still-at-the-top.md") {
		t.Errorf("plan does not list the move:\n%s", out)
	}
	sameFiles(t, snapshot(t, root), before)
}

func TestMoveRefusesAnUnknownFolderAndAMissingRef(t *testing.T) {
	root := copyTree(t, mixedFixture)
	if _, err := runIn(t, root, "move", "4", "--to", "archive"); err == nil {
		t.Errorf("moving to an unknown folder succeeded")
	}
	if _, err := runIn(t, root, "move"); err == nil {
		t.Errorf("move with no documents and no --all succeeded")
	}
}

func TestSetTypeHintsAtMoveOrMovesWithTheFlag(t *testing.T) {
	root := copyTree(t, folderedFixture)

	out := runInOK(t, root, "set", "3", "type", "reference")
	if !strings.Contains(out, "docs/task/003-Loader-review.md now belongs in docs/ref/: run `wj move 3`") {
		t.Errorf("set did not point at wj move:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "task", "003-Loader-review.md")); err != nil {
		t.Errorf("set moved the file without --move")
	}

	runInOK(t, root, "set", "2", "type", "reference", "--move")
	if _, err := os.Stat(filepath.Join(root, "docs", "ref", "002-Ship-the-loader.md")); err != nil {
		t.Fatalf("set --move did not move 002 into ref/")
	}
	if got := read(t, filepath.Join(root, "docs", "ref", "001-Design.md")); !strings.Contains(got, "[002](002-Ship-the-loader.md#plan)") {
		t.Errorf("the link into the moved 002 was not rewritten:\n%s", got)
	}
}

func TestSetMoveInAFlatProjectDoesNotMove(t *testing.T) {
	root := copyFixture(t)
	runInOK(t, root, "set", "4", "type", "reference", "--move")
	if _, err := os.Stat(filepath.Join(root, "docs", "04-Ship-the-loader.md")); err != nil {
		t.Errorf("set --move moved a doc in a flat project")
	}
}
