package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestTheLintHookPathspecReachesTheTypeFolders(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return string(out)
	}
	git("init", "--initial-branch=master")

	for _, name := range []string{"docs/000-Docs.md", "docs/ref/001-Design.md", "docs/task/002-Ship.md"} {
		absPath := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absPath, []byte("# doc\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("add", "docs")

	pathspec := regexp.MustCompile(`-- '([^']+)'`).FindStringSubmatch(LintHook)[1]
	staged := strings.Fields(git("diff", "--cached", "--name-only", "--diff-filter=ACMR", "--", pathspec))
	if strings.Join(staged, " ") != "docs/000-Docs.md docs/ref/001-Design.md docs/task/002-Ship.md" {
		t.Errorf("the hook would lint %v", staged)
	}
}
