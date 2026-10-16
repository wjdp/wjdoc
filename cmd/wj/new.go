package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/wjdp/wjdoc/internal/docs"
	"github.com/wjdp/wjdoc/internal/gitx"
	"github.com/wjdp/wjdoc/internal/render"
)

var createdStyle = lipgloss.NewStyle().Bold(true)

func newCommand(shared *app) *cobra.Command {
	return &cobra.Command{
		Use:   "new <type> <title words...>",
		Short: "Create the next numbered document",
		Long: `Create a document in docs/ with the next free number.

<type> is reference, review, task or bug; an unrecognised type is allowed and
lints as a warning. The remaining arguments are the title, joined with spaces,
and become both the slug and the ` + "`# `" + ` heading. Tasks, bugs and reviews get a status
(todo, and open for reviews); references get none.

The number is one past the highest in use, counted over docs/ on every local and
remote git branch so parallel branches do not collide. Padding width is taken
from the directory; when the width is exhausted the command refuses and points
at wj repad.

Examples:
  wj new task Ship the loader      writes docs/007-Ship-the-loader.md
  wj new bug List crashes on empty docs
  wj new reference Prisma to Drizzle
  wj new task Try it out --json    prints {"number":7,"path":"docs/..."}`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			documents, err := docs.Load(shared.root)
			if err != nil {
				return err
			}

			elsewhere, err := gitx.DocNamesAcrossBranches(shared.root)
			if err != nil {
				return err
			}

			docType, title := args[0], strings.Join(args[1:], " ")
			number := docs.NextNumber(documents, elsewhere)

			path, err := docs.Create(shared.root, number, docs.Width(documents), docType, title)
			if err != nil {
				return err
			}

			out := command.OutOrStdout()
			switch shared.mode() {
			case render.JSON:
				encoder := json.NewEncoder(out)
				encoder.SetIndent("", "  ")
				return encoder.Encode(struct {
					Number int    `json:"number"`
					Path   string `json:"path"`
				}{number, path})
			case render.Pretty:
				fmt.Fprintln(out, createdStyle.Render("created "+path))
			default:
				fmt.Fprintln(out, path)
			}
			return nil
		},
	}
}
