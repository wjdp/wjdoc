package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const fixtureRoot = "../../internal/docs/testdata/basic"

// run executes the root command against the fixture project and returns
// everything it wrote, plus any error.
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()

	out := &bytes.Buffer{}
	command := rootCommand()
	command.SetOut(out)
	command.SetErr(out)
	command.SetArgs(append([]string{"-C", fixtureRoot}, args...))

	err := command.Execute()
	return out.String(), err
}

func runOK(t *testing.T, args ...string) string {
	t.Helper()

	out, err := run(t, args...)
	if err != nil {
		t.Fatalf("wj %s: %v", strings.Join(args, " "), err)
	}
	return out
}

func lines(out string) []string {
	trimmed := strings.TrimRight(out, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func TestListPlainHasOneTabbedLinePerFile(t *testing.T) {
	got := lines(runOK(t, "list", "--plain"))

	const fixtureFiles = 9
	if len(got) != fixtureFiles {
		t.Fatalf("got %d lines, want %d:\n%s", len(got), fixtureFiles, strings.Join(got, "\n"))
	}
	for _, line := range got {
		if fields := strings.Split(line, "\t"); len(fields) != 6 {
			t.Errorf("line %q has %d tab-separated fields, want 6", line, len(fields))
		}
	}
	if !strings.Contains(got[len(got)-1], "docs/README.md") {
		t.Errorf("last line %q does not list README.md", got[len(got)-1])
	}
}

func TestListFiltersByType(t *testing.T) {
	got := lines(runOK(t, "list", "--plain", "--type", "task"))

	want := []string{
		"docs/01-Sync robustness.md",
		"docs/04-Ship-the-loader.md",
		"docs/05-Duplicate-number.md",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d:\n%s", len(got), len(want), strings.Join(got, "\n"))
	}
	for index, line := range got {
		if !strings.HasPrefix(line, want[index]+"\t") {
			t.Errorf("line %d is %q, want it to start with %q", index, line, want[index])
		}
	}
}

func TestListJSONCarriesTheStableFields(t *testing.T) {
	var rows []map[string]any
	if err := json.Unmarshal([]byte(runOK(t, "list", "--json")), &rows); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(rows) != 9 {
		t.Fatalf("got %d rows, want 9", len(rows))
	}

	for _, key := range []string{"number", "path", "type", "status", "title", "created", "modified"} {
		if _, present := rows[0][key]; !present {
			t.Errorf("first row has no %q key", key)
		}
	}
	if rows[0]["path"] != "docs/00-Docs.md" || rows[0]["type"] != "reference" {
		t.Errorf("unexpected first row: %v", rows[0])
	}
}

func TestShowByNumberPrintsTheFile(t *testing.T) {
	got := runOK(t, "show", "1", "--plain")

	for _, want := range []string{"type: task", "# Sync robustness", "space in the filename"} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not contain %q:\n%s", want, got)
		}
	}
}

func TestShowBodyDropsTheFrontmatter(t *testing.T) {
	got := runOK(t, "show", "1", "--plain", "--body")

	if strings.Contains(got, "type: task") {
		t.Errorf("--body kept the frontmatter:\n%s", got)
	}
	if !strings.Contains(got, "# Sync robustness") {
		t.Errorf("--body dropped the heading:\n%s", got)
	}
}

func TestShowAmbiguousFragmentFails(t *testing.T) {
	_, err := run(t, "show", "loader")
	if err == nil {
		t.Fatal("ambiguous reference did not fail")
	}
	for _, want := range []string{"ambiguous", "docs/04-Ship-the-loader.md", "docs/05-Loader-review.md"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q: %v", want, err)
		}
	}
}

func TestConventionsDescribesType(t *testing.T) {
	got := runOK(t, "conventions")

	if !strings.Contains(got, "type") {
		t.Errorf("conventions does not mention type:\n%s", got)
	}
	if count := len(lines(got)); count > 30 {
		t.Errorf("conventions is %d lines, want at most 30", count)
	}
}

// datedFixture copies the fixture project into a temp git repository and
// commits each document at a chosen age in days, so that the git-derived
// columns and filters have something to work on.
func datedFixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	root := t.TempDir()
	gitAt(t, root, "init", "--initial-branch=master")
	gitAt(t, root, "config", "user.email", "test@example.com")
	gitAt(t, root, "config", "user.name", "Test")

	docsDir := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(fixtureRoot, "docs"))
	if err != nil {
		t.Fatal(err)
	}

	// Newest last: each file is committed on its own, older files first, so
	// that the ages below are distinct.
	ages := map[string]int{
		"01-Sync robustness.md":  2,
		"04-Ship-the-loader.md":  40,
		"05-Duplicate-number.md": 300,
	}
	commits := make([]string, 0, len(entries))
	for _, entry := range entries {
		commits = append(commits, entry.Name())
	}
	sort.Slice(commits, func(i, j int) bool {
		return ageOf(ages, commits[i]) > ageOf(ages, commits[j])
	})

	for _, name := range commits {
		source, err := os.ReadFile(filepath.Join(fixtureRoot, "docs", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(docsDir, name), source, 0o644); err != nil {
			t.Fatal(err)
		}
		gitAt(t, root, "add", filepath.Join("docs", name))
		when := time.Now().AddDate(0, 0, -ageOf(ages, name)).Format(time.RFC3339)
		commit := exec.Command("git", "-C", root, "commit", "-m", "add "+name)
		commit.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+when, "GIT_COMMITTER_DATE="+when)
		if out, err := commit.CombinedOutput(); err != nil {
			t.Fatalf("git commit: %v: %s", err, out)
		}
	}
	return root
}

// ageOf is the age in days a fixture file is committed at, defaulting to a
// year for the files no test cares about.
func ageOf(ages map[string]int, name string) int {
	if age, chosen := ages[name]; chosen {
		return age
	}
	return 365
}

func paths(out string) []string {
	listed := []string{}
	for _, line := range lines(out) {
		listed = append(listed, strings.Split(line, "\t")[0])
	}
	return listed
}

func TestListSortByModifiedIsNewestFirst(t *testing.T) {
	root := datedFixture(t)

	out, err := runAt(t, root, "list", "--plain", "--sort", "modified")
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	got := paths(out)
	want := []string{"docs/01-Sync robustness.md", "docs/04-Ship-the-loader.md"}
	for index, path := range want {
		if got[index] != path {
			t.Fatalf("row %d is %q, want %q:\n%s", index, got[index], path, out)
		}
	}
	if got[len(got)-1] == want[0] {
		t.Errorf("--reverse would be needed for this order:\n%s", out)
	}

	reversed, err := runAt(t, root, "list", "--plain", "--sort", "modified", "--reverse")
	if err != nil {
		t.Fatalf("list --reverse: %v", err)
	}
	if last := paths(reversed); last[len(last)-1] != want[0] {
		t.Errorf("--reverse did not flip the order:\n%s", reversed)
	}
}

func TestListSinceKeepsRecentlyModifiedDocuments(t *testing.T) {
	root := datedFixture(t)

	out, err := runAt(t, root, "list", "--plain", "--since", "1w")
	if err != nil {
		t.Fatalf("list --since: %v", err)
	}

	got := paths(out)
	if len(got) != 1 || got[0] != "docs/01-Sync robustness.md" {
		t.Errorf("--since 1w listed %v, want just the newest document", got)
	}

	wider, err := runAt(t, root, "list", "--plain", "--since", "2mo")
	if err != nil {
		t.Fatalf("list --since 2mo: %v", err)
	}
	if len(paths(wider)) != 2 {
		t.Errorf("--since 2mo listed %v, want two documents", paths(wider))
	}

	if _, err := runAt(t, root, "list", "--since", "yesterday"); err == nil {
		t.Error("an unparseable --since did not fail")
	}
}

func TestListActiveDropsFinishedDocuments(t *testing.T) {
	root := datedFixture(t)

	out, err := runAt(t, root, "list", "--plain", "--active")
	if err != nil {
		t.Fatalf("list --active: %v", err)
	}

	for _, line := range lines(out) {
		fields := strings.Split(line, "\t")
		if fields[2] == "done" || fields[2] == "superseded" {
			t.Errorf("--active kept %q", line)
		}
	}
	if !strings.Contains(out, "docs/01-Sync robustness.md") {
		t.Errorf("--active dropped an in-progress document:\n%s", out)
	}
	if strings.Contains(out, "docs/05-Duplicate-number.md") {
		t.Errorf("--active kept a done document:\n%s", out)
	}
}
