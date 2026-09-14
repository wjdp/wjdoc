package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const lintFixtureRoot = "../../internal/lint/testdata/lint"

func runLint(t *testing.T, args ...string) (string, error) {
	t.Helper()

	out := &bytes.Buffer{}
	command := rootCommand()
	command.SetOut(out)
	command.SetErr(out)
	command.SetArgs(append([]string{"-C", lintFixtureRoot, "lint"}, args...))

	err := command.Execute()
	return out.String(), err
}

func TestLintPlainReportsFindingsAndFails(t *testing.T) {
	out, err := runLint(t, "--plain")
	if err == nil {
		t.Fatal("a fixture full of errors did not fail")
	}

	want := []string{
		"docs/001-Good.md:4: error: link target docs/099-Missing.md does not exist",
		"docs/002-No-frontmatter.md:1: error: frontmatter is missing",
		"docs/11-Narrow-padding.md: warning: padding width 2 differs from the project width of 3",
	}
	for _, line := range want {
		if !strings.Contains(out, line+"\n") {
			t.Errorf("output does not contain %q:\n%s", line, out)
		}
	}
	for _, line := range lines(out) {
		if !strings.Contains(line, ": error: ") && !strings.Contains(line, ": warning: ") {
			t.Errorf("unexpected line %q", line)
		}
	}
}

func TestLintNarrowsToTheGivenPaths(t *testing.T) {
	out, _ := runLint(t, "--plain", "docs/002-No-frontmatter.md")

	got := lines(out)
	if len(got) != 1 || !strings.HasPrefix(got[0], "docs/002-No-frontmatter.md:1: error: frontmatter is missing") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestLintAcceptsABareFilename(t *testing.T) {
	out, err := runLint(t, "--plain", "11-Narrow-padding.md")
	if err != nil {
		t.Fatalf("a warning-only selection should exit 0: %v", err)
	}
	if len(lines(out)) != 1 || !strings.Contains(out, "warning: padding width 2") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestLintJSONCarriesTheFields(t *testing.T) {
	out, err := runLint(t, "--json", "docs/001-Good.md")
	if err == nil {
		t.Fatal("a broken link did not fail")
	}

	var rows []map[string]any
	if unmarshalErr := json.Unmarshal([]byte(out), &rows); unmarshalErr != nil {
		t.Fatalf("unmarshal: %v\n%s", unmarshalErr, out)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2:\n%s", len(rows), out)
	}
	for _, key := range []string{"path", "line", "level", "message"} {
		if _, present := rows[0][key]; !present {
			t.Errorf("first row has no %q key", key)
		}
	}
}

func TestLintIsSilentOnACleanSelection(t *testing.T) {
	out, err := runLint(t, "--plain", "docs/000-About.md")
	if err != nil {
		t.Fatalf("a clean selection failed: %v", err)
	}
	if out != "" {
		t.Errorf("clean plain output is not empty:\n%s", out)
	}
}
