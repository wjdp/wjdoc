package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if output, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	return dir
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()

	path := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// linkedProject is two documents, one linking to the other and to a document
// that is not there, plus a README mentioning the target from outside docs/.
func linkedProject(t *testing.T) string {
	t.Helper()

	dir := gitRepo(t)
	writeFile(t, dir, "docs/001-Target.md", "---\ntype: reference\n---\n\n# Target\n")
	writeFile(t, dir, "docs/002-Source.md",
		"---\ntype: task\nstatus: todo\n---\n\n# Source\n\nSee [target](001-Target.md) and [gone](009-Gone.md).\n")
	writeFile(t, dir, "README.md", "Read docs/001-Target.md first.\n")
	return dir
}

func TestLinksResolvesTargetsAndMarksMissing(t *testing.T) {
	got := lines(runInOK(t, linkedProject(t), "links", "2", "--plain"))

	want := []string{"docs/001-Target.md", "docs/009-Gone.md missing"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for index, line := range got {
		if line != want[index] {
			t.Errorf("line %d is %q, want %q", index, line, want[index])
		}
	}
}

func TestLinksJSONCarriesTargetLineAndExists(t *testing.T) {
	var rows []struct {
		Target string `json:"target"`
		Line   int    `json:"line"`
		Exists bool   `json:"exists"`
	}
	if err := json.Unmarshal([]byte(runInOK(t, linkedProject(t), "links", "2", "--json")), &rows); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %v", len(rows), rows)
	}
	if rows[0].Target != "docs/001-Target.md" || !rows[0].Exists {
		t.Errorf("unexpected first row: %v", rows[0])
	}
	if rows[0].Line != 8 {
		t.Errorf("first row is on line %d, want 8: the frontmatter is not counted", rows[0].Line)
	}
	if rows[1].Exists {
		t.Errorf("missing target reported as existing: %v", rows[1])
	}
}

func TestBacklinksReportPathAndLine(t *testing.T) {
	got := lines(runInOK(t, linkedProject(t), "backlinks", "1", "--plain"))

	if !contains(got, "docs/002-Source.md:8") {
		t.Errorf("got %v, want docs/002-Source.md:8: the frontmatter is not counted", got)
	}
	if _, err := exec.LookPath("rg"); err == nil && !contains(got, "README.md:1") {
		t.Errorf("got %v, want README.md:1 from the repository scan", got)
	}
}

func TestInitCreatesEverythingThenChangesNothing(t *testing.T) {
	dir := gitRepo(t)

	first := runInOK(t, dir, "init")
	for _, want := range []string{"created docs/", "created docs/000-Docs.md", "created AGENTS.md", "created .git/hooks/pre-commit"} {
		if !strings.Contains(first, want) {
			t.Errorf("first run does not report %q:\n%s", want, first)
		}
	}

	about := read(t, filepath.Join(dir, "docs", "000-Docs.md"))
	for _, want := range []string{"type: reference", "# About these docs", "wj conventions"} {
		if !strings.Contains(about, want) {
			t.Errorf("about-doc does not contain %q:\n%s", want, about)
		}
	}
	agents := read(t, filepath.Join(dir, "AGENTS.md"))
	if !strings.Contains(agents, "Run wj conventions") {
		t.Errorf("AGENTS.md does not carry the line:\n%s", agents)
	}
	hook := read(t, filepath.Join(dir, ".git", "hooks", "pre-commit"))
	if !strings.Contains(hook, "wj lint") || !strings.Contains(hook, "--cached") {
		t.Errorf("hook does not lint staged documents:\n%s", hook)
	}

	second := runInOK(t, dir, "init")
	for _, want := range []string{"exists docs/", "exists docs/000-Docs.md", "exists AGENTS.md", "exists .git/hooks/pre-commit"} {
		if !strings.Contains(second, want) {
			t.Errorf("second run does not report %q:\n%s", want, second)
		}
	}
	if read(t, filepath.Join(dir, "AGENTS.md")) != agents {
		t.Error("AGENTS.md changed on the second run")
	}
	if read(t, filepath.Join(dir, ".git", "hooks", "pre-commit")) != hook {
		t.Error("the hook changed on the second run")
	}

	if out, err := runIn(t, dir, "lint", "--plain"); err != nil {
		t.Errorf("the initialised project does not lint: %v\n%s", err, out)
	}
}

func TestInitLeavesAForeignHookAlone(t *testing.T) {
	dir := gitRepo(t)
	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	const foreign = "#!/bin/sh\nmake check\n"
	writeFile(t, dir, ".git/hooks/pre-commit", foreign)

	got := runInOK(t, dir, "init")

	if !strings.Contains(got, "skipped .git/hooks/pre-commit (foreign hook)") {
		t.Errorf("foreign hook not reported:\n%s", got)
	}
	if !strings.Contains(got, "wj lint") {
		t.Errorf("no snippet printed for the user to add:\n%s", got)
	}
	if read(t, hookPath) != foreign {
		t.Error("the foreign hook was overwritten")
	}
}

func TestInitUsesCLAUDEWhenOnlyThatExists(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, dir, "CLAUDE.md", "# Project\n")

	if got := runInOK(t, dir, "init"); !strings.Contains(got, "appended CLAUDE.md") {
		t.Errorf("CLAUDE.md not used:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
		t.Error("AGENTS.md was created alongside CLAUDE.md")
	}
}

func TestInitKeepsAnExistingAboutDocAndWidth(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, dir, "docs/00-Docs.md", "---\ntype: reference\n---\n\n# About these docs\n")

	got := runInOK(t, dir, "init")

	if !strings.Contains(got, "exists docs/00-Docs.md") {
		t.Errorf("existing about-doc not kept:\n%s", got)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
