package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTagAddsAndRemovesInFlowStyle(t *testing.T) {
	root := copyFixture(t)
	path := filepath.Join(root, "docs", "04-Ship-the-loader.md")

	runInOK(t, root, "tag", "4", "area:engine", "Reach:App", "area:engine", "--plain")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "tags: [area:engine, reach:app]\n") {
		t.Errorf("tags not written as one flow line:\n%s", source)
	}

	runInOK(t, root, "tag", "4", "--remove", "area:engine", "reach:app", "--plain")
	source, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(source), "tags") {
		t.Errorf("removing the last tag left the key:\n%s", source)
	}
}

func TestTagRefusesABadlyShapedTag(t *testing.T) {
	root := copyFixture(t)
	if _, err := runIn(t, root, "tag", "4", "two words"); err == nil {
		t.Fatal("tag accepted a tag with a space")
	}
}

func TestTagsCountsByNamespace(t *testing.T) {
	root := copyFixture(t)
	runInOK(t, root, "tag", "4", "area:engine", "reach:app", "loose")
	runInOK(t, root, "tag", "1", "area:engine", "area:web")

	got := runInOK(t, root, "tags", "--plain")
	want := "2\tarea:engine\n1\tarea:web\n1\treach:app\n1\tloose\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestListFiltersByTagAndPriority(t *testing.T) {
	root := copyFixture(t)
	runInOK(t, root, "tag", "4", "area:engine")
	runInOK(t, root, "tag", "1", "area:web")
	runInOK(t, root, "set", "1", "priority", "p1")

	cases := []struct {
		args []string
		want []string
	}{
		{[]string{"--tag", "area:engine"}, []string{"docs/04-Ship-the-loader.md"}},
		{[]string{"--tag", "area:"}, []string{"docs/01-Sync robustness.md", "docs/04-Ship-the-loader.md"}},
		{[]string{"--tag", "area:", "--tag", "area:web"}, []string{"docs/01-Sync robustness.md"}},
		{[]string{"-p", "P1"}, []string{"docs/01-Sync robustness.md"}},
	}
	for _, testCase := range cases {
		t.Run(strings.Join(testCase.args, " "), func(t *testing.T) {
			got := lines(runInOK(t, root, append([]string{"list", "--plain"}, testCase.args...)...))
			if len(got) != len(testCase.want) {
				t.Fatalf("got %d rows, want %d:\n%s", len(got), len(testCase.want), strings.Join(got, "\n"))
			}
			for index, want := range testCase.want {
				if !strings.HasPrefix(got[index], want+"\t") {
					t.Errorf("row %d is %q, want %s", index, got[index], want)
				}
			}
		})
	}
}

func TestNextRanksByPriorityWithinStatus(t *testing.T) {
	root := copyFixture(t)
	runInOK(t, root, "new", "task", "Urgent thing", "--plain")
	runInOK(t, root, "new", "task", "Lesser thing", "--plain")
	runInOK(t, root, "set", "Urgent", "priority", "p0")
	runInOK(t, root, "set", "Lesser", "priority", "p3")

	var todo []string
	for _, line := range lines(runInOK(t, root, "next", "--plain")) {
		if fields := strings.Split(line, "\t"); fields[2] == "todo" {
			todo = append(todo, fields[3])
		}
	}
	want := []string{"Urgent thing", "Lesser thing", "Ship the loader"}
	if strings.Join(todo, "|") != strings.Join(want, "|") {
		t.Errorf("todo order is %q, want %q", todo, want)
	}
}
