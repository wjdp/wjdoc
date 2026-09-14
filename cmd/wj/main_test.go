package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
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
		if fields := strings.Split(line, "\t"); len(fields) != 4 {
			t.Errorf("line %q has %d tab-separated fields, want 4", line, len(fields))
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

	for _, key := range []string{"number", "path", "type", "status", "title", "modified"} {
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
