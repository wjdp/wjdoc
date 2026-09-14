package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wjdp/wjdoc/internal/docs"
	"github.com/wjdp/wjdoc/internal/links"
)

// repadProject writes a two-digit project with references to one of its docs
// from a sibling doc, an agent file and a source file.
func repadProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	if output, err := exec.Command("git", "-C", root, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}

	files := map[string]string{
		"docs/00-Docs.md":  "---\ntype: reference\n---\n\n# Docs\n",
		"docs/07-Thing.md": "---\ntype: task\nstatus: todo\n---\n\n# Thing\n",
		"docs/08-Other.md": "---\ntype: task\nstatus: todo\n---\n\n# Other\n\nSee [Thing](07-Thing.md).\n",
		"docs/12-Wide.md":  "---\ntype: task\nstatus: todo\n---\n\n# Wide\n",
		"CLAUDE.md":        "Read docs/07-Thing.md before touching the loader.\n",
		"src/a.ts":         "// see 07-Thing.md\nexport const a = 1;\n",
	}
	for name, content := range files {
		absPath := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func runRepad(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()

	out := &bytes.Buffer{}
	command := rootCommand()
	command.SetOut(out)
	command.SetErr(out)
	command.SetArgs(append([]string{"-C", root}, args...))

	err := command.Execute()
	return out.String(), err
}

func TestRepadDryRunListsRenamesAndEdits(t *testing.T) {
	root := repadProject(t)

	out, err := runRepad(t, root, "repad", "--width", "3", "--dry-run")
	if err != nil {
		t.Fatalf("repad --dry-run: %v", err)
	}

	renames, edits := 0, 0
	for _, line := range lines(out) {
		if strings.HasPrefix(line, "docs/") && !strings.Contains(line, ": ") {
			renames++
			continue
		}
		if strings.Contains(line, " → ") {
			edits++
		}
	}
	if renames < 2 {
		t.Errorf("got %d renames, want at least 2:\n%s", renames, out)
	}
	if edits != 3 {
		t.Errorf("got %d edits, want 3:\n%s", edits, out)
	}

	if _, err := os.Stat(filepath.Join(root, "docs", "07-Thing.md")); err != nil {
		t.Errorf("--dry-run renamed a file: %v", err)
	}
}

func TestRepadApplyRewritesEveryReference(t *testing.T) {
	root := repadProject(t)

	out, err := runRepad(t, root, "repad", "--width", "3")
	if err != nil {
		t.Fatalf("repad: %v", err)
	}
	if !strings.Contains(out, "review and commit") {
		t.Errorf("no summary line:\n%s", out)
	}

	for _, name := range []string{"docs/000-Docs.md", "docs/007-Thing.md", "docs/008-Other.md", "docs/012-Wide.md"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			t.Errorf("%s missing after repad: %v", name, err)
		}
	}

	for name, want := range map[string]string{
		"CLAUDE.md": "docs/007-Thing.md",
		"src/a.ts":  "see 007-Thing.md",
	} {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), want) {
			t.Errorf("%s does not contain %q:\n%s", name, want, content)
		}
	}

	documents, err := docs.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, document := range documents {
		for _, link := range links.Extract(document.Body) {
			target := links.ResolveTarget(document.Path, link.Target)
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(target))); err != nil {
				t.Errorf("%s:%d: broken link to %s", document.Path, link.Line, target)
			}
		}
	}
}

func TestRepadRefusesATooNarrowWidth(t *testing.T) {
	root := repadProject(t)

	if _, err := runRepad(t, root, "repad", "--width", "1"); err == nil {
		t.Fatal("width 1 did not fail")
	}
}
