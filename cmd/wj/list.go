package main

import (
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/wjdp/wjdoc/internal/docs"
	"github.com/wjdp/wjdoc/internal/render"
)

var listTitleStyle = lipgloss.NewStyle().Bold(true)

func listCommand(shared *app) *cobra.Command {
	var types, statuses []string

	command := &cobra.Command{
		Use:   "list",
		Short: "List the documents in docs/",
		Long: `List every document in docs/, optionally filtered by frontmatter type or status.

Filters take a comma-separated list, may be repeated, and match case-insensitively.
Documents whose filename does not match the convention are listed too, so that
nothing in docs/ is hidden.

On a terminal: a bold "<project> docs · N files" line, then a coloured table of
document, type, status and, at 120 columns or wider, title.
Piped, or with --plain: one line per document, tab-separated, no header and no
colour: path, type, status, title. Absent fields are empty.
With --json: an array of objects with number, path, type, status, title,
modified and error.

Examples:
  wj list                              every document
  wj list --type task --status todo    just the todo tasks
  wj list --status todo,in-progress    either status
  wj list --json                       the same rows as JSON`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			documents, err := docs.Load(shared.root)
			if err != nil {
				return err
			}
			documents = docs.Filter(documents, types, statuses)

			out := command.OutOrStdout()
			mode := shared.mode()
			if mode == render.Pretty {
				fmt.Fprintln(out, listTitleStyle.Render(fmt.Sprintf("%s docs · %d %s", filepath.Base(shared.root), len(documents), plural(len(documents), "file", "files"))))
			}
			render.List(out, documents, mode, terminalWidth())
			return nil
		},
	}

	command.Flags().StringSliceVar(&types, "type", nil, "keep only these types (comma-separated, repeatable)")
	command.Flags().StringSliceVar(&statuses, "status", nil, "keep only these statuses (comma-separated, repeatable)")
	return command
}

func plural(count int, singular, pluralForm string) string {
	if count == 1 {
		return singular
	}
	return pluralForm
}
