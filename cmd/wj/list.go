package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

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
	var all, reverse, active, showPaths bool
	var sortBy, since string

	command := &cobra.Command{
		Use:   "list",
		Short: "List the documents in docs/",
		Long: `List every document in docs/, optionally filtered by frontmatter type or status.

Filters take a comma-separated list, may be repeated, and match case-insensitively.
Documents whose filename does not match the convention are listed too, so that
nothing in docs/ is hidden.

The Created and Modified columns come from git: the commit that first added the
file under any name, and the last commit to change its content. A pure rename
does not count as a change. Both are relative ages on a terminal and ISO dates
elsewhere, and are empty outside a git repository.

--sort orders by number (the default), created, modified or status. created and
modified put the newest first; status follows the order of the footer counts and
then the number. --reverse flips whichever order is in force.

--since keeps documents modified within a window, given as a Go duration (72h)
or with a d, w, mo or y suffix (2w). --active drops the done and superseded
documents. Both compose with --type, --status and --all.

--all lists every sibling project instead of just this one: each directory with a
docs/ subdirectory in the parent of the project root, or in $` + projectsEnv + ` when that is
set. Paths are then prefixed with the project directory name and filters apply
across all of them.

On a terminal: a bold "<project> docs · N files" line, then a coloured table of
number, status, title, type, created and modified, and a faint line of counts by
status. The number links to the file; unnumbered files show their filename there.
Narrow terminals drop Type, then Created, then Modified to keep the title
readable, and the title is truncated to whatever space is left. Done and
superseded rows are faint. --paths adds a Document column of the repo-relative
path, which is never dropped.
Piped, or with --plain: one line per document, tab-separated, no header and no
colour: path, type, status, title, created, modified. Absent fields are empty.
With --json: an array of objects with number, path, type, status, title, created,
modified and error.

Examples:
  wj list                              every document
  wj list --type task --status todo    just the todo tasks
  wj list --status todo,in-progress    either status
  wj list --active --sort modified     what is live, most recently touched first
  wj list --since 2w                   touched in the last fortnight
  wj list --all --status in-progress   what is in flight across every project
  wj list --json                       the same rows as JSON`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			documents, heading, err := listDocuments(shared, all)
			if err != nil {
				return err
			}
			documents = docs.Filter(documents, types, statuses)
			if active {
				documents = activeOnly(documents)
			}
			if since != "" {
				window, err := parseSince(since)
				if err != nil {
					return err
				}
				documents = modifiedWithin(documents, window)
			}
			if err := sortDocuments(documents, sortBy, reverse); err != nil {
				return err
			}

			out := command.OutOrStdout()
			mode := shared.mode()
			if mode == render.Pretty {
				fmt.Fprintln(out, listTitleStyle.Render(heading(len(documents))))
			}
			render.List(out, documents, mode, terminalWidth(), showPaths)
			return nil
		},
	}

	command.Flags().StringSliceVar(&types, "type", nil, "keep only these types (comma-separated, repeatable)")
	command.Flags().StringSliceVar(&statuses, "status", nil, "keep only these statuses (comma-separated, repeatable)")
	command.Flags().BoolVar(&all, "all", false, "list every sibling project, not just this one")
	command.Flags().StringVar(&sortBy, "sort", sortByNumber, "order by `key`: number, created, modified or status")
	command.Flags().BoolVar(&reverse, "reverse", false, "reverse the order")
	command.Flags().StringVar(&since, "since", "", "keep documents modified within a `window`, e.g. 3d, 2w, 6mo, 72h")
	command.Flags().BoolVar(&active, "active", false, "drop the done and superseded documents")
	command.Flags().BoolVar(&showPaths, "paths", false, "add a column of repo-relative paths")
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

// loadWithDates loads a project's documents and fills in their git dates.
func loadWithDates(root string) ([]docs.Document, error) {
	documents, err := docs.Load(root)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(documents))
	for _, document := range documents {
		paths = append(paths, document.Path)
	}
	dates, err := gitx.DocumentDates(root, paths)
	if err != nil {
		return documents, nil
	}
	for index := range documents {
		documents[index].Created = dates[documents[index].Path].Created
		documents[index].Modified = dates[documents[index].Path].Modified
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

// The keys --sort accepts.
const (
	sortByNumber   = "number"
	sortByCreated  = "created"
	sortByModified = "modified"
	sortByStatus   = "status"
)

// statusOrder ranks statuses for --sort status, matching the order of the
// listing's footer counts. Unknown statuses sort after the known ones,
// alphabetically, and statusless references sort last.
var statusOrder = map[string]int{
	"in-progress": 0,
	"blocked":     1,
	"todo":        2,
	"planned":     3,
	"open":        4,
	"done":        5,
	"superseded":  6,
}

func statusRank(status string) int {
	if status == "" {
		return len(statusOrder) + 1
	}
	if rank, known := statusOrder[status]; known {
		return rank
	}
	return len(statusOrder)
}

func sortDocuments(documents []docs.Document, key string, reverse bool) error {
	// Files that do not follow the numbering convention have no number to sort
	// on, so they sit at the end.
	less := func(left, right docs.Document) bool {
		if (left.Number < 0) != (right.Number < 0) {
			return right.Number < 0
		}
		return left.Number < right.Number
	}
	switch key {
	case sortByNumber:
	case sortByCreated:
		less = func(left, right docs.Document) bool { return left.Created.After(right.Created) }
	case sortByModified:
		less = func(left, right docs.Document) bool { return left.Modified.After(right.Modified) }
	case sortByStatus:
		less = func(left, right docs.Document) bool {
			if statusRank(left.Status) != statusRank(right.Status) {
				return statusRank(left.Status) < statusRank(right.Status)
			}
			if left.Status != right.Status {
				return left.Status < right.Status
			}
			return left.Number < right.Number
		}
	default:
		return fmt.Errorf("unknown --sort %q: want number, created, modified or status", key)
	}

	sort.SliceStable(documents, func(i, j int) bool {
		if reverse {
			return less(documents[j], documents[i])
		}
		return less(documents[i], documents[j])
	})
	return nil
}

func activeOnly(documents []docs.Document) []docs.Document {
	kept := make([]docs.Document, 0, len(documents))
	for _, document := range documents {
		if document.Status == "done" || document.Status == "superseded" {
			continue
		}
		kept = append(kept, document)
	}
	return kept
}

func modifiedWithin(documents []docs.Document, window time.Duration) []docs.Document {
	kept := make([]docs.Document, 0, len(documents))
	for _, document := range documents {
		if document.Modified.IsZero() || time.Since(document.Modified) > window {
			continue
		}
		kept = append(kept, document)
	}
	return kept
}

// sincePattern matches the calendar suffixes Go durations lack.
var sincePattern = regexp.MustCompile(`^(\d+)(d|w|mo|y)$`)

// parseSince reads a window as a Go duration (72h) or with a day, week, month
// or year suffix (3d, 2w, 6mo, 1y).
func parseSince(value string) (time.Duration, error) {
	if match := sincePattern.FindStringSubmatch(strings.TrimSpace(value)); match != nil {
		count, err := strconv.Atoi(match[1])
		if err != nil {
			return 0, fmt.Errorf("invalid --since %q", value)
		}
		days := map[string]int{"d": 1, "w": 7, "mo": 30, "y": 365}[match[2]]
		return time.Duration(count*days) * 24 * time.Hour, nil
	}
	window, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("invalid --since %q: want a duration such as 3d, 2w, 6mo or 72h", value)
	}
	return window, nil
}
