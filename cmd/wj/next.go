package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/wjdp/wjdoc/internal/docs"
	"github.com/wjdp/wjdoc/internal/render"
)

var nextTitleStyle = lipgloss.NewStyle().Bold(true)

// nextOrder ranks the actionable statuses; anything else is excluded.
var nextOrder = map[string]int{
	"in-progress": 0,
	"blocked":     1,
	"todo":        2,
	"planned":     3,
}

var blockedReasonPattern = regexp.MustCompile(`(?im)^Blocked(?: by)?:.*$`)

func nextCommand(shared *app) *cobra.Command {
	var showPaths bool

	command := &cobra.Command{
		Use:   "next",
		Short: "List the actionable documents",
		Long: `List the documents worth picking up next: status in-progress, then
blocked, then todo, then planned, ordered by number within each group.
Everything else (open reviews, shelved, done, rejected, superseded,
references, and unknown statuses) is excluded.

For a blocked document, when its body has a line starting with "Blocked:" or
"Blocked by:" (case-insensitive), that line is appended to the Title column
as "<title> — <reason>" in every mode, plain and JSON included.

On a terminal: a bold "next up · N" line, then the same table as wj list, with
--paths adding the same Document column. The ordering is fixed: wj list's
--sort, --since and --active do not apply here.
Piped, or with --plain: the same tab-separated rows as wj list.
With --json: the same array shape as wj list.

Run this at the start of a session to see what to work on.

Examples:
  wj next                          what to pick up next
  wj next --plain                  the same, for scripts
  wj -C ~/local/grate next --json  another project's queue, as JSON`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			documents, err := loadWithDates(shared.root)
			if err != nil {
				return err
			}

			actionable := make([]docs.Document, 0, len(documents))
			for _, document := range documents {
				if _, ok := nextOrder[document.Status]; !ok {
					continue
				}
				actionable = append(actionable, withBlockedReason(document))
			}
			sort.SliceStable(actionable, func(i, j int) bool {
				left, right := actionable[i], actionable[j]
				if nextOrder[left.Status] != nextOrder[right.Status] {
					return nextOrder[left.Status] < nextOrder[right.Status]
				}
				return left.Number < right.Number
			})

			out := command.OutOrStdout()
			mode := shared.mode()
			if mode == render.Pretty {
				fmt.Fprintln(out, nextTitleStyle.Render(fmt.Sprintf("next up · %d", len(actionable))))
			}
			render.List(out, actionable, mode, terminalWidth(), showPaths)
			return nil
		},
	}

	command.Flags().BoolVar(&showPaths, "paths", false, "add a column of repo-relative paths")
	return command
}

// withBlockedReason returns a copy of document with its Title extended by a
// "Blocked:" or "Blocked by:" line from the body, when the document is
// blocked and such a line is present.
func withBlockedReason(document docs.Document) docs.Document {
	if document.Status != "blocked" {
		return document
	}
	reason := blockedReasonPattern.FindString(document.Body)
	if reason == "" {
		return document
	}
	document.Title = document.Title + " — " + strings.TrimSpace(reason)
	return document
}
