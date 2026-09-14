package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
}

func git(t *testing.T, root string, args ...string) {
	t.Helper()
	if _, err := Run(root, args...); err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
}

func writeDoc(t *testing.T, root, name string) {
	t.Helper()
	docsDir := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, name), []byte("# doc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// repository builds a repo with 001 on master and 002 on a side branch that is
// not checked out.
func repository(t *testing.T) string {
	t.Helper()
	requireGit(t)

	root := t.TempDir()
	git(t, root, "init", "--initial-branch=master")
	git(t, root, "config", "user.email", "test@example.com")
	git(t, root, "config", "user.name", "Test")

	writeDoc(t, root, "001-First.md")
	git(t, root, "add", "docs/001-First.md")
	git(t, root, "commit", "-m", "first")

	git(t, root, "checkout", "-b", "side")
	writeDoc(t, root, "002-Second.md")
	git(t, root, "add", "docs/002-Second.md")
	git(t, root, "commit", "-m", "second")
	git(t, root, "checkout", "master")

	return root
}

func TestRunReturnsTrimmedOutput(t *testing.T) {
	root := repository(t)

	branch, err := Run(root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if branch != "master" {
		t.Errorf("got %q, want %q", branch, "master")
	}
}

func TestRunFoldsStderrIntoTheError(t *testing.T) {
	root := repository(t)

	_, err := Run(root, "rev-parse", "--verify", "no-such-ref")
	if err == nil {
		t.Fatal("expected an error")
	}
	if len(err.Error()) == 0 {
		t.Error("error has no message")
	}
}

func TestDocNamesAcrossBranchesSeesOtherBranches(t *testing.T) {
	root := repository(t)

	names, err := DocNamesAcrossBranches(root)
	if err != nil {
		t.Fatalf("DocNamesAcrossBranches: %v", err)
	}

	want := []string{"001-First.md", "002-Second.md"}
	if len(names) != len(want) {
		t.Fatalf("got %v, want %v", names, want)
	}
	for index, name := range names {
		if name != want[index] {
			t.Errorf("got %v, want %v", names, want)
		}
	}
}

func TestDocNamesOutsideARepositoryIsEmpty(t *testing.T) {
	requireGit(t)

	names, err := DocNamesAcrossBranches(t.TempDir())
	if err != nil {
		t.Fatalf("DocNamesAcrossBranches: %v", err)
	}
	if names != nil {
		t.Errorf("got %v, want nil", names)
	}
}

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	docsDir := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// historyRepository builds a linear repo whose single doc changes status from
// todo to done in its second commit.
func historyRepository(t *testing.T) string {
	t.Helper()
	requireGit(t)

	root := t.TempDir()
	git(t, root, "init", "--initial-branch=master")
	git(t, root, "config", "user.email", "test@example.com")
	git(t, root, "config", "user.name", "Test")

	writeFile(t, root, "001-First.md", "---\ntype: task\nstatus: todo\n---\n\n# First\n")
	git(t, root, "add", "docs/001-First.md")
	git(t, root, "commit", "-m", "add first")

	writeFile(t, root, "001-First.md", "---\ntype: task\nstatus: done\n---\n\n# First\n")
	git(t, root, "add", "docs/001-First.md")
	git(t, root, "commit", "-m", "finish first")

	return root
}

func TestModifiedDatesCoversEveryTrackedDoc(t *testing.T) {
	root := repository(t)

	dates, err := ModifiedDates(root, []string{"docs/001-First.md", "docs/002-Second.md"})
	if err != nil {
		t.Fatalf("ModifiedDates: %v", err)
	}
	if _, present := dates["docs/001-First.md"]; !present {
		t.Errorf("no date for the committed doc: %v", dates)
	}
	if _, present := dates["docs/002-Second.md"]; present {
		t.Errorf("dated a doc that is not on this branch: %v", dates)
	}
}

func TestModifiedDatesOutsideARepositoryIsEmpty(t *testing.T) {
	requireGit(t)

	dates, err := ModifiedDates(t.TempDir(), []string{"docs/001-First.md"})
	if err != nil {
		t.Fatalf("ModifiedDates: %v", err)
	}
	if len(dates) != 0 {
		t.Errorf("got %v, want an empty map", dates)
	}
}

func TestHistoryIsNewestFirst(t *testing.T) {
	root := historyRepository(t)

	commits, err := History(root, "docs/001-First.md")
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2: %v", len(commits), commits)
	}
	if commits[0].Subject != "finish first" || commits[1].Subject != "add first" {
		t.Errorf("wrong order: %v", commits)
	}
	if commits[0].Short == "" || len(commits[0].Hash) != 40 || commits[0].Date.IsZero() {
		t.Errorf("incomplete commit: %+v", commits[0])
	}
}

func TestStatusAtReadsTheFrontmatter(t *testing.T) {
	root := historyRepository(t)

	commits, err := History(root, "docs/001-First.md")
	if err != nil {
		t.Fatalf("History: %v", err)
	}

	for index, want := range []string{"done", "todo"} {
		status, err := StatusAt(root, commits[index].Hash, "docs/001-First.md")
		if err != nil {
			t.Fatalf("StatusAt: %v", err)
		}
		if status != want {
			t.Errorf("commit %d: got %q, want %q", index, status, want)
		}
	}
}

func TestStatusAtOfAMissingFileIsEmpty(t *testing.T) {
	root := historyRepository(t)

	status, err := StatusAt(root, "HEAD", "docs/999-Absent.md")
	if err != nil {
		t.Fatalf("StatusAt: %v", err)
	}
	if status != "" {
		t.Errorf("got %q, want empty", status)
	}
}
