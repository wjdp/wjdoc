package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/wjdp/wjdoc/internal/docs"
	"github.com/wjdp/wjdoc/internal/gitx"
	"github.com/wjdp/wjdoc/internal/render"
)

var listTitleStyle = lipgloss.NewStyle().Bold(true)

// projectsEnv names the directory --all walks, overriding the parent of the
// project root.
const projectsEnv = "WJ_PROJECTS"

func listCommand(shared *app) *cobra.Command {
	var types, statuses []string
	var all bool

	command := &cobra.Command{
		Use:   "list",
		Short: "List the documents in docs/",
		Long: `List every document in docs/, optionally filtered by frontmatter type or status.

Filters take a comma-separated list, may be repeated, and match case-insensitively.
Documents whose filename does not match the convention are listed too, so that
nothing in docs/ is hidden.

The Modified column is the date of the last commit touching the file, shown as a
relative age on a terminal and as an ISO date elsewhere. It is empty outside a
git repository.

--all lists every sibling project instead of just this one: each directory with a
docs/ subdirectory in the parent of the project root, or in $` + projectsEnv + ` when that is
set. Paths are then prefixed with the project directory name and filters apply
across all of them.

On a terminal: a bold "<project> docs · N files" line, then a coloured table of
document, type, status, modified and, at 120 columns or wider, title.
Piped, or with --plain: one line per document, tab-separated, no header and no
colour: path, type, status, title, modified. Absent fields are empty.
With --json: an array of objects with number, path, type, status, title,
modified and error.

Examples:
  wj list                              every document
  wj list --type task --status todo    just the todo tasks
  wj list --status todo,in-progress    either status
  wj list --all --status in-progress   what is in flight across every project
  wj list --json                       the same rows as JSON`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			documents, heading, err := listDocuments(shared, all)
			if err != nil {
				return err
			}
			documents = docs.Filter(documents, types, statuses)

			out := command.OutOrStdout()
			mode := shared.mode()
			if mode == render.Pretty {
				fmt.Fprintln(out, listTitleStyle.Render(heading(len(documents))))
			}
			render.List(out, documents, mode, terminalWidth())
			return nil
		},
	}

	command.Flags().StringSliceVar(&types, "type", nil, "keep only these types (comma-separated, repeatable)")
	command.Flags().StringSliceVar(&statuses, "status", nil, "keep only these statuses (comma-separated, repeatable)")
	command.Flags().BoolVar(&all, "all", false, "list every sibling project, not just this one")
	return command
}

// listDocuments loads either this project or every sibling project, along with
// the pretty-mode title line for a given number of listed documents.
func listDocuments(shared *app, all bool) ([]docs.Document, func(int) string, error) {
	if all {
		documents, projects, err := loadAllProjects(shared.root)
		if err != nil {
			return nil, nil, err
		}
		heading := func(count int) string {
			return fmt.Sprintf("%s · %d %s · %d %s", filepath.Base(projectsDir(shared.root)),
				projects, plural(projects, "project", "projects"), count, plural(count, "file", "files"))
		}
		return documents, heading, nil
	}

	documents, err := loadWithDates(shared.root)
	if err != nil {
		return nil, nil, err
	}
	heading := func(count int) string {
		return fmt.Sprintf("%s docs · %d %s", filepath.Base(shared.root), count, plural(count, "file", "files"))
	}
	return documents, heading, nil
}

// loadWithDates loads a project's documents and fills in Modified from git.
func loadWithDates(root string) ([]docs.Document, error) {
	documents, err := docs.Load(root)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(documents))
	for _, document := range documents {
		paths = append(paths, document.Path)
	}
	dates, err := gitx.ModifiedDates(root, paths)
	if err != nil {
		return documents, nil
	}
	for index := range documents {
		documents[index].Modified = dates[documents[index].Path]
	}
	return documents, nil
}

// projectsDir is the directory --all walks: $WJ_PROJECTS when set, otherwise
// the parent of the project root. (docs/001-Design.md leaves this open; it is
// resolved here as the parent, with the environment variable as an override.)
func projectsDir(root string) string {
	if override := os.Getenv(projectsEnv); override != "" {
		return override
	}
	return filepath.Dir(root)
}

// loadAllProjects loads every sibling project with a docs/ directory, with
// each document's path prefixed by its project directory name.
func loadAllProjects(root string) ([]docs.Document, int, error) {
	parent := projectsDir(root)
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil, 0, err
	}

	combined := []docs.Document{}
	projects := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		projectRoot := filepath.Join(parent, entry.Name())
		if info, err := os.Stat(filepath.Join(projectRoot, "docs")); err != nil || !info.IsDir() {
			continue
		}
		documents, err := loadWithDates(projectRoot)
		if err != nil || !anyNumbered(documents) {
			continue
		}
		projects++
		for _, document := range documents {
			document.Path = entry.Name() + "/" + document.Path
			combined = append(combined, document)
		}
	}
	return combined, projects, nil
}

func plural(count int, singular, pluralForm string) string {
	if count == 1 {
		return singular
	}
	return pluralForm
}

func anyNumbered(documents []docs.Document) bool {
	for _, document := range documents {
		if document.Number >= 0 {
			return true
		}
	}
	return false
}
