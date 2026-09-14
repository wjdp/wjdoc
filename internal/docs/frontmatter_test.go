package docs

import (
	"strings"
	"testing"
)

const sample = "---\ntype: task\n# a comment\nstatus: todo\n---\n\n# Heading\n\nBody with --- in it\nand a trailing line.\n"

func TestSetFieldReplacesOneLineAndLeavesTheBody(t *testing.T) {
	got, err := SetField([]byte(sample), "status", "done")
	if err != nil {
		t.Fatalf("SetField: %v", err)
	}

	want := "---\ntype: task\n# a comment\nstatus: done\n---\n\n# Heading\n\nBody with --- in it\nand a trailing line.\n"
	if string(got) != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}

	_, body := splitFrontmatter(string(got))
	_, originalBody := splitFrontmatter(sample)
	if body != originalBody {
		t.Errorf("body changed:\n%q\nwant:\n%q", body, originalBody)
	}
}

func TestSetFieldAppendsAMissingKey(t *testing.T) {
	got, err := SetField([]byte("---\ntype: reference\n---\n\n# Heading\n"), "status", "done")
	if err != nil {
		t.Fatalf("SetField: %v", err)
	}

	want := "---\ntype: reference\nstatus: done\n---\n\n# Heading\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSetFieldCreatesAMissingBlock(t *testing.T) {
	got, err := SetField([]byte("# Heading\n\nBody.\n"), "type", "task")
	if err != nil {
		t.Fatalf("SetField: %v", err)
	}

	want := "---\ntype: task\n---\n\n# Heading\n\nBody.\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSetFieldQuotesOnlyWhenNeeded(t *testing.T) {
	cases := []struct{ value, want string }{
		{"done", "status: done"},
		{"a: b", `status: "a: b"`},
		{"*star", `status: "*star"`},
		{"", `status: ""`},
		{"multi word value", "status: multi word value"},
	}

	for _, testCase := range cases {
		t.Run(testCase.value, func(t *testing.T) {
			got, err := SetField([]byte("---\nstatus: todo\n---\n"), "status", testCase.value)
			if err != nil {
				t.Fatalf("SetField: %v", err)
			}
			if !strings.Contains(string(got), testCase.want+"\n") {
				t.Errorf("got %q, want it to contain %q", got, testCase.want)
			}
		})
	}
}

func TestSetFieldPreservesCRLF(t *testing.T) {
	got, err := SetField([]byte("---\r\ntype: task\r\n---\r\n\r\n# Heading\r\n"), "status", "done")
	if err != nil {
		t.Fatalf("SetField: %v", err)
	}

	want := "---\r\ntype: task\r\nstatus: done\r\n---\r\n\r\n# Heading\r\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestUnsetFieldRemovesOnlyThatLine(t *testing.T) {
	got, err := UnsetField([]byte(sample), "status")
	if err != nil {
		t.Fatalf("UnsetField: %v", err)
	}

	want := "---\ntype: task\n# a comment\n---\n\n# Heading\n\nBody with --- in it\nand a trailing line.\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestUnsetFieldIsAnNoOpWhenAbsent(t *testing.T) {
	for _, source := range []string{sample, "# No frontmatter\n"} {
		got, err := UnsetField([]byte(source), "author")
		if err != nil {
			t.Fatalf("UnsetField: %v", err)
		}
		if string(got) != source {
			t.Errorf("got %q, want it unchanged", got)
		}
	}
}
