package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// copyFixture copies the basic fixture into a temporary directory so that the
// writing commands never touch testdata.
func copyFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	docsDir := filepath.Join(root, "docs")
	if err := os.Mkdir(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(filepath.Join(fixtureRoot, "docs"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		source, err := os.ReadFile(filepath.Join(fixtureRoot, "docs", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(docsDir, entry.Name()), source, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// runIn is run from main_test.go against an arbitrary project root.
func runIn(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()

	out := &bytes.Buffer{}
	command := rootCommand()
	command.SetOut(out)
	command.SetErr(out)
	command.SetArgs(append([]string{"-C", root}, args...))

	err := command.Execute()
	return out.String(), err
}

func runInOK(t *testing.T, root string, args ...string) string {
	t.Helper()

	out, err := runIn(t, root, args...)
	if err != nil {
		t.Fatalf("wj %s: %v", strings.Join(args, " "), err)
	}
	return out
}

func read(t *testing.T, path string) string {
	t.Helper()

	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(source)
}

func TestNewAllocatesTheNextNumberAtTheFixtureWidth(t *testing.T) {
	root := copyFixture(t)

	got := runInOK(t, root, "new", "task", "Try", "it", "out", "--plain")

	const want = "docs/07-Try-it-out.md"
	if strings.TrimSpace(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	const wantSource = "---\ntype: task\nstatus: todo\n---\n\n# Try it out\n"
	if source := read(t, filepath.Join(root, "docs", "07-Try-it-out.md")); source != wantSource {
		t.Errorf("got %q, want %q", source, wantSource)
	}
}

func TestNewJSONCarriesNumberAndPath(t *testing.T) {
	root := copyFixture(t)

	var created struct {
		Number int    `json:"number"`
		Path   string `json:"path"`
	}
	if err := json.Unmarshal([]byte(runInOK(t, root, "new", "review", "Loader", "again", "--json")), &created); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if created.Number != 7 || created.Path != "docs/07-Loader-again.md" {
		t.Errorf("got %+v", created)
	}
}

func TestNewRefusesWhenThePaddingIsExhausted(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "99-Last.md"), []byte("---\ntype: task\nstatus: todo\n---\n\n# Last\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := runIn(t, root, "new", "task", "One too many")
	if err == nil {
		t.Fatal("new did not refuse an overflowing number")
	}
	if !strings.Contains(err.Error(), "wj repad --width 3") {
		t.Errorf("error does not point at repad: %v", err)
	}

	entries, readErr := os.ReadDir(filepath.Join(root, "docs"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 1 {
		t.Errorf("new wrote a file despite refusing: %d entries", len(entries))
	}
}

func TestSetChangesOnlyTheStatusLine(t *testing.T) {
	root := copyFixture(t)
	path := filepath.Join(root, "docs", "04-Ship-the-loader.md")
	before := read(t, path)

	if got := runInOK(t, root, "set", "4", "status", "done", "--plain"); got != "" {
		t.Errorf("set printed %q, want nothing", got)
	}

	want := strings.Replace(before, "status: todo", "status: done", 1)
	if after := read(t, path); after != want {
		t.Errorf("got %q, want %q", after, want)
	}
}

func TestSetUnsetRemovesTheKey(t *testing.T) {
	root := copyFixture(t)
	path := filepath.Join(root, "docs", "04-Ship-the-loader.md")

	runInOK(t, root, "set", "4", "status", "--unset", "--plain")

	if after := read(t, path); strings.Contains(after, "status:") {
		t.Errorf("status survived --unset:\n%s", after)
	}
}

func TestSetAddsAMissingKey(t *testing.T) {
	root := copyFixture(t)
	path := filepath.Join(root, "docs", "00-Docs.md")

	runInOK(t, root, "set", "0", "status", "done", "--plain")

	const want = "---\ntype: reference\nstatus: done\n---\n\n# About these docs\n\nFixture about-doc.\n"
	if after := read(t, path); after != want {
		t.Errorf("got %q, want %q", after, want)
	}
}
