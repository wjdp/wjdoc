package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runAt executes the root command against an arbitrary project directory.
func runAt(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()

	out := &bytes.Buffer{}
	command := rootCommand()
	command.SetOut(out)
	command.SetErr(out)
	command.SetArgs(append([]string{"-C", dir}, args...))

	err := command.Execute()
	return out.String(), err
}

func gitAt(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// statusChangeRepository builds a repo whose one doc moves from todo to done.
func statusChangeRepository(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	root := t.TempDir()
	gitAt(t, root, "init", "--initial-branch=master")
	gitAt(t, root, "config", "user.email", "test@example.com")
	gitAt(t, root, "config", "user.name", "Test")

	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(status string) {
		body := "---\ntype: task\nstatus: " + status + "\n---\n\n# First\n"
		if err := os.WriteFile(filepath.Join(root, "docs", "001-First.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("todo")
	gitAt(t, root, "add", "docs/001-First.md")
	gitAt(t, root, "commit", "-m", "add first")

	write("done")
	gitAt(t, root, "add", "docs/001-First.md")
	gitAt(t, root, "commit", "-m", "finish first")

	return root
}

func TestHistoryPlainShowsTheStatusChange(t *testing.T) {
	root := statusChangeRepository(t)

	out, err := runAt(t, root, "history", "1", "--plain")
	if err != nil {
		t.Fatalf("history: %v", err)
	}

	got := lines(out)
	if len(got) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(got), out)
	}
	newest := strings.Split(got[0], "\t")
	if len(newest) != 4 {
		t.Fatalf("line %q has %d fields, want 4", got[0], len(newest))
	}
	if newest[2] != "finish first" {
		t.Errorf("newest subject is %q, want %q", newest[2], "finish first")
	}
	if newest[3] != "todo -> done" {
		t.Errorf("status change is %q, want %q", newest[3], "todo -> done")
	}
	if oldest := strings.Split(got[1], "\t"); oldest[3] != "none -> todo" {
		t.Errorf("oldest status change is %q, want %q", oldest[3], "none -> todo")
	}
}

func TestHistoryJSONCarriesTheStableFields(t *testing.T) {
	root := statusChangeRepository(t)

	out, err := runAt(t, root, "history", "1", "--json")
	if err != nil {
		t.Fatalf("history: %v", err)
	}

	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	for _, key := range []string{"hash", "date", "subject", "status_from", "status_to"} {
		if _, present := rows[0][key]; !present {
			t.Errorf("first row has no %q key", key)
		}
	}
	if rows[0]["status_from"] != "todo" || rows[0]["status_to"] != "done" {
		t.Errorf("unexpected status fields: %v", rows[0])
	}
}

func TestListShowsTheGitDates(t *testing.T) {
	root := statusChangeRepository(t)

	out, err := runAt(t, root, "list", "--plain")
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	fields := strings.Split(strings.TrimRight(out, "\n"), "\t")
	if len(fields) != 8 {
		t.Fatalf("got %d fields, want 8: %q", len(fields), out)
	}
	for _, index := range []int{4, 5} {
		if len(fields[index]) != len("2006-01-02") {
			t.Errorf("field %d is %q, want an ISO date", index, fields[index])
		}
	}
}

func TestListAllPrefixesPathsWithTheProject(t *testing.T) {
	root := statusChangeRepository(t)
	t.Setenv("WJ_PROJECTS", filepath.Dir(root))

	out, err := runAt(t, root, "list", "--all", "--plain")
	if err != nil {
		t.Fatalf("list --all: %v", err)
	}

	want := filepath.Base(root) + "/docs/001-First.md\t"
	if !strings.HasPrefix(out, want) {
		t.Errorf("output does not start with %q:\n%s", want, out)
	}
}
