package gitx

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestDocumentDatesCoversEveryTrackedDoc(t *testing.T) {
	root := repository(t)

	dates, err := DocumentDates(root, []string{"docs/001-First.md", "docs/002-Second.md"})
	if err != nil {
		t.Fatalf("DocumentDates: %v", err)
	}
	first, present := dates["docs/001-First.md"]
	if !present {
		t.Fatalf("no dates for the committed doc: %v", dates)
	}
	if first.Created.IsZero() || first.Modified.IsZero() {
		t.Errorf("incomplete dates: %+v", first)
	}
	if _, present := dates["docs/002-Second.md"]; present {
		t.Errorf("dated a doc that is not on this branch: %v", dates)
	}
}

func TestDocumentDatesOutsideARepositoryIsEmpty(t *testing.T) {
	requireGit(t)

	dates, err := DocumentDates(t.TempDir(), []string{"docs/001-First.md"})
	if err != nil {
		t.Fatalf("DocumentDates: %v", err)
	}
	if len(dates) != 0 {
		t.Errorf("got %v, want an empty map", dates)
	}
}

// commitAt commits the staged changes with both dates fixed, so that tests can
// assert on exact commit times.
func commitAt(t *testing.T, root, when, message string) {
	t.Helper()
	command := exec.Command("git", "-C", root, "commit", "-m", message)
	command.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+when, "GIT_COMMITTER_DATE="+when)
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, out)
	}
}

func day(t *testing.T, iso string) string {
	t.Helper()
	return iso + "T12:00:00+00:00"
}

// longBody is a document long enough that a one-line edit still reads as a
// rename to git's similarity detection.
func longBody(variant string) string {
	lines := []string{"# First", ""}
	for index := range 12 {
		lines = append(lines, fmt.Sprintf("Paragraph %d of the document.", index))
	}
	return strings.Join(append(lines, variant, ""), "\n")
}

// renameRepository writes one doc, edits it, renames it untouched, then renames
// it again while editing it, each commit on a known day.
func renameRepository(t *testing.T) string {
	t.Helper()
	requireGit(t)

	root := t.TempDir()
	git(t, root, "init", "--initial-branch=master")
	git(t, root, "config", "user.email", "test@example.com")
	git(t, root, "config", "user.name", "Test")

	writeFile(t, root, "001-First.md", longBody("one"))
	git(t, root, "add", "docs/001-First.md")
	commitAt(t, root, day(t, "2026-01-01"), "add first")

	writeFile(t, root, "001-First.md", longBody("two"))
	git(t, root, "add", "docs/001-First.md")
	commitAt(t, root, day(t, "2026-02-01"), "edit first")

	git(t, root, "mv", "docs/001-First.md", "docs/001-Renamed.md")
	commitAt(t, root, day(t, "2026-03-01"), "pure rename")

	git(t, root, "mv", "docs/001-Renamed.md", "docs/001-Renamed-again.md")
	writeFile(t, root, "001-Renamed-again.md", longBody("three"))
	git(t, root, "add", "docs/001-Renamed-again.md")
	commitAt(t, root, day(t, "2026-04-01"), "rename and edit")

	writeFile(t, root, "002-Untouched.md", "# Second\n")
	git(t, root, "add", "docs/002-Untouched.md")
	commitAt(t, root, day(t, "2026-05-01"), "add second")

	return root
}

func TestDocumentDatesFollowsRenamesAndIgnoresPureOnes(t *testing.T) {
	root := renameRepository(t)

	dates, err := DocumentDates(root, []string{"docs/001-Renamed-again.md", "docs/002-Untouched.md"})
	if err != nil {
		t.Fatalf("DocumentDates: %v", err)
	}

	renamed := dates["docs/001-Renamed-again.md"]
	if got := renamed.Created.Format("2006-01-02"); got != "2026-01-01" {
		t.Errorf("created %q, want the first commit under the original name", got)
	}
	if got := renamed.Modified.Format("2006-01-02"); got != "2026-04-01" {
		t.Errorf("modified %q, want the rename that also edited the file", got)
	}

	second := dates["docs/002-Untouched.md"]
	if got := second.Created.Format("2006-01-02"); got != "2026-05-01" {
		t.Errorf("created %q, want its only commit", got)
	}
	if !second.Created.Equal(second.Modified) {
		t.Errorf("a doc committed once has created %v and modified %v", second.Created, second.Modified)
	}
}

func TestDocumentDatesIgnoresAPureRenameAsAModification(t *testing.T) {
	root := renameRepository(t)

	// Before the fourth commit the newest change to the doc is the pure rename
	// of 2026-03-01, which must not count; the edit of 2026-02-01 does.
	git(t, root, "checkout", "HEAD~2")
	dates, err := DocumentDates(root, []string{"docs/001-Renamed.md"})
	if err != nil {
		t.Fatalf("DocumentDates: %v", err)
	}
	renamed := dates["docs/001-Renamed.md"]
	if got := renamed.Modified.Format("2006-01-02"); got != "2026-02-01" {
		t.Errorf("modified %q, want the last content change", got)
	}
	if got := renamed.Created.Format("2006-01-02"); got != "2026-01-01" {
		t.Errorf("created %q, want the first commit", got)
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
