package lint

import (
	"strings"
	"testing"

	"github.com/wjdp/wjdoc/internal/docs"
)

const fixtureRoot = "testdata/lint"

func runFixture(t *testing.T) []Finding {
	t.Helper()

	documents, err := docs.Load(fixtureRoot)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return Run(fixtureRoot, documents)
}

func format(finding Finding) string {
	return finding.Path + ": " + finding.Level.String() + ": " + finding.Message
}

func TestRunReportsEveryRule(t *testing.T) {
	findings := runFixture(t)

	cases := []struct {
		name  string
		path  string
		level Level
		want  string
	}{
		{"missing frontmatter", "docs/002-No-frontmatter.md", Error, "frontmatter is missing"},
		{"unparseable frontmatter", "docs/004-Broken-yaml.md", Error, "frontmatter is not valid YAML"},
		{"missing type", "docs/005-No-type.md", Error, "type is missing"},
		{"missing status on a review", "docs/009-Review-without-status.md", Error, "status is missing"},
		{"duplicate number", "docs/010-Duplicate.md", Error, "number 10 is also used by docs/010-Duplicate-again.md"},
		{"duplicate number, the other way", "docs/010-Duplicate-again.md", Error, "number 10 is also used by docs/010-Duplicate.md"},
		{"lowercase filename", "docs/012-lowercase-title.md", Error, "filename does not follow"},
		{"unnumbered filename", "docs/README.md", Error, "filename does not follow"},
		{"spaces in the filename", "docs/013-Space in name.md", Error, "filename contains spaces, slugs use hyphens"},
		{"open status on a task", "docs/014-Open-task.md", Warning, `status "open" is only for review`},
		{"missing status on a bug", "docs/016-Bug-without-status.md", Error, "status is missing, required for a bug"},
		{"broken link", "docs/001-Good.md", Error, "link target docs/099-Missing.md does not exist"},
		{"unknown type", "docs/006-Unknown-type.md", Warning, `unknown type "diagram"`},
		{"unknown status", "docs/007-Unknown-status.md", Warning, `unknown status "nearly"`},
		{"status on a reference", "docs/008-Reference-with-status.md", Warning, "status is not used on a reference"},
		{"mixed padding width", "docs/11-Narrow-padding.md", Warning, "padding width 2 differs from the project width of 3"},
		{"link to a superseded doc", "docs/001-Good.md", Warning, "which is superseded"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			for _, finding := range findings {
				if finding.Path == testCase.path && finding.Level == testCase.level && strings.Contains(finding.Message, testCase.want) {
					return
				}
			}
			t.Errorf("no %s for %s containing %q in:\n%s", testCase.level, testCase.path, testCase.want, join(findings))
		})
	}
}

func TestRunIsSilentOnConformingDocuments(t *testing.T) {
	for _, finding := range runFixture(t) {
		if finding.Path == "docs/000-About.md" || finding.Path == "docs/003-Superseded.md" || finding.Path == "docs/017-Good-bug.md" {
			t.Errorf("unexpected finding: %s", format(finding))
		}
	}
}

func TestRunHasNoOpinionOnUnknownTypeStatus(t *testing.T) {
	for _, finding := range runFixture(t) {
		if finding.Path == "docs/006-Unknown-type.md" && strings.Contains(finding.Message, "status") {
			t.Errorf("unknown type should not be judged on status: %s", format(finding))
		}
	}
}

func TestRunSkipsLinksInFencedCode(t *testing.T) {
	for _, finding := range runFixture(t) {
		if strings.Contains(finding.Message, "098-Also-missing.md") {
			t.Errorf("link inside a fenced block was linted: %s", format(finding))
		}
	}
}

func TestRunSortsByPathThenLine(t *testing.T) {
	findings := runFixture(t)
	for index := 1; index < len(findings); index++ {
		previous, current := findings[index-1], findings[index]
		if previous.Path > current.Path || (previous.Path == current.Path && previous.Line > current.Line) {
			t.Errorf("findings %d and %d are out of order:\n%s\n%s", index-1, index, format(previous), format(current))
		}
	}
}

func TestHasErrors(t *testing.T) {
	if !HasErrors(runFixture(t)) {
		t.Error("the fixture has errors, HasErrors says otherwise")
	}
	if HasErrors([]Finding{{Level: Warning}}) {
		t.Error("warnings alone are not errors")
	}
	if HasErrors(nil) {
		t.Error("no findings are not errors")
	}
}

func join(findings []Finding) string {
	lines := make([]string, 0, len(findings))
	for _, finding := range findings {
		lines = append(lines, format(finding))
	}
	return strings.Join(lines, "\n")
}

func TestRunFlagsAPatternAndSpaceProblemTogether(t *testing.T) {
	var messages []string
	for _, finding := range runFixture(t) {
		if finding.Path == "docs/015-both problems.md" {
			messages = append(messages, finding.Message)
		}
	}
	if len(messages) != 2 {
		t.Fatalf("got %d findings, want both the pattern and the space error: %v", len(messages), messages)
	}
}

func TestRunAcceptsOpenOnAReview(t *testing.T) {
	documents := []docs.Document{{Path: "docs/001-Review.md", Number: 1, Width: 3, Type: "review", Status: "open"}}
	for _, finding := range Run(t.TempDir(), documents) {
		if strings.Contains(finding.Message, `status "open"`) {
			t.Errorf("open is valid on a review: %s", format(finding))
		}
	}
}

func TestRunNarrowsShelvedAndRejectedToTasksAndBugs(t *testing.T) {
	documents := []docs.Document{
		{Path: "docs/001-Task.md", Number: 1, Width: 3, Type: "task", Status: "shelved"},
		{Path: "docs/002-Bug.md", Number: 2, Width: 3, Type: "bug", Status: "rejected"},
		{Path: "docs/003-Review.md", Number: 3, Width: 3, Type: "review", Status: "shelved"},
	}
	var flagged []string
	for _, finding := range Run(t.TempDir(), documents) {
		flagged = append(flagged, finding.Path+": "+finding.Message)
	}
	want := []string{`docs/003-Review.md: status "shelved" is only for task and bug`}
	if strings.Join(flagged, "\n") != strings.Join(want, "\n") {
		t.Errorf("got %q, want %q", flagged, want)
	}
}

func TestRunFlagsTheFilenamePatternAndSpacesSeparately(t *testing.T) {
	findings := runFixture(t)
	count := 0
	for _, finding := range findings {
		if finding.Path == "docs/013-Space in name.md" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("got %d findings for the spaced filename, want 1:\n%s", count, join(findings))
	}
}
