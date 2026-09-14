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
