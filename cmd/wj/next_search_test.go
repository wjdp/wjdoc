package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestNextOrdersAndExcludes(t *testing.T) {
	got := lines(runOK(t, "next", "--plain"))

	want := []string{
		"docs/01-Sync robustness.md",
		"docs/04-Ship-the-loader.md",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d:\n%s", len(got), len(want), strings.Join(got, "\n"))
	}
	for index, line := range got {
		if !strings.HasPrefix(line, want[index]+"\t") {
			t.Errorf("line %d is %q, want it to start with %q", index, line, want[index])
		}
	}

	out := strings.Join(got, "\n")
	for _, excluded := range []string{"docs/05-Duplicate-number.md", "docs/05-Loader-review.md", "docs/00-Docs.md"} {
		if strings.Contains(out, excluded) {
			t.Errorf("next included %q, which is not actionable", excluded)
		}
	}
}

func TestSearchPlainFindsTheTerm(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("rg not on PATH")
	}

	got := runOK(t, "search", "robustness", "--plain")

	if !strings.Contains(got, `docs/01-Sync robustness.md:6:# Sync robustness`) {
		t.Errorf("search output missing expected match:\n%s", got)
	}
}

func TestNextPathsAddsTheDocumentColumn(t *testing.T) {
	out := runOK(t, "next", "--paths", "--plain")

	if !strings.Contains(out, "docs/01-Sync robustness.md") {
		t.Errorf("--paths changed the plain rows:\n%s", out)
	}
}
