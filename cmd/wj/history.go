package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/wjdp/wjdoc/internal/docs"
	"github.com/wjdp/wjdoc/internal/gitx"
	"github.com/wjdp/wjdoc/internal/render"
)

var (
	historyTitleStyle  = lipgloss.NewStyle().Bold(true)
	historyHashStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	historyDateStyle   = lipgloss.NewStyle().Faint(true)
	historyChangeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
)

// historyEntry is one commit touching a document, with the frontmatter status
// it left behind and the one its parent had.
type historyEntry struct {
	Hash       string `json:"hash"`
	Short      string `json:"-"`
	Date       string `json:"date"`
	Subject    string `json:"subject"`
	StatusFrom string `json:"status_from"`
	StatusTo   string `json:"status_to"`
}

func (e historyEntry) changed() bool { return e.StatusFrom != e.StatusTo }

func historyCommand(shared *app) *cobra.Command {
	command := &cobra.Command{
		Use:   "history <ref>",
		Short: "List the commits touching a document",
		Long: `List the commits that touched a document, newest first, following renames.

A commit whose frontmatter status differs from its parent's is annotated with
the change, so the life of a task is readable at a glance.

<ref> is an integer, a padded number, a slug fragment or a path, as for wj show.

On a terminal: a bold header, then short hash, date, subject and any status
change, coloured.
Piped, or with --plain: one line per commit, tab-separated: short hash, date,
subject, status change ("todo -> done", empty when unchanged).
With --json: an array of objects with hash, date, subject, status_from and
status_to.

Examples:
  wj history 1                  the life of doc 1
  wj history design --plain     the same, for scripts
  wj history 7 --json           as JSON`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			documents, err := docs.Load(shared.root)
			if err != nil {
				return err
			}
			document, err := docs.Resolve(documents, args[0])
			if err != nil {
				return err
			}

			entries, err := documentHistory(shared.root, document.Path)
			if err != nil {
				return err
			}

			out := command.OutOrStdout()
			switch shared.mode() {
			case render.JSON:
				encoder := json.NewEncoder(out)
				encoder.SetIndent("", "  ")
				return encoder.Encode(entries)
			case render.Plain:
				historyPlain(out, entries)
			default:
				fmt.Fprintln(out, historyTitleStyle.Render(fmt.Sprintf("%s · %d %s",
					document.Path, len(entries), plural(len(entries), "commit", "commits"))))
				historyPretty(out, entries)
			}
			return nil
		},
	}
	return command
}

// documentHistory pairs each commit with the status it left and the status its
// parent had, reading each commit's copy of the file exactly once.
func documentHistory(root, path string) ([]historyEntry, error) {
	commits, err := gitx.History(root, path)
	if err != nil {
		return nil, err
	}

	statuses := make([]string, len(commits))
	for index, commit := range commits {
		status, err := gitx.StatusAt(root, commit.Hash, path)
		if err != nil {
			return nil, err
		}
		statuses[index] = status
	}

	entries := make([]historyEntry, 0, len(commits))
	for index, commit := range commits {
		parentStatus := ""
		if index+1 < len(commits) {
			parentStatus = statuses[index+1]
		}
		entries = append(entries, historyEntry{
			Hash:       commit.Hash,
			Short:      commit.Short,
			Date:       commit.Date.Format("2006-01-02"),
			Subject:    commit.Subject,
			StatusFrom: parentStatus,
			StatusTo:   statuses[index],
		})
	}
	return entries, nil
}

func historyPlain(w io.Writer, entries []historyEntry) {
	for _, entry := range entries {
		change := ""
		if entry.changed() {
			change = fmt.Sprintf("%s -> %s", displayStatus(entry.StatusFrom), displayStatus(entry.StatusTo))
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", entry.Short, entry.Date, entry.Subject, change)
	}
}

func historyPretty(w io.Writer, entries []historyEntry) {
	shortWidth := 0
	for _, entry := range entries {
		if width := lipgloss.Width(entry.Short); width > shortWidth {
			shortWidth = width
		}
	}

	for _, entry := range entries {
		line := &strings.Builder{}
		fmt.Fprintf(line, "%s  %s  %s",
			historyHashStyle.Render(entry.Short+strings.Repeat(" ", shortWidth-len(entry.Short))),
			historyDateStyle.Render(entry.Date),
			entry.Subject)
		if entry.changed() {
			fmt.Fprintf(line, "  %s", historyChangeStyle.Render(fmt.Sprintf("status: %s → %s",
				displayStatus(entry.StatusFrom), displayStatus(entry.StatusTo))))
		}
		fmt.Fprintln(w, line.String())
	}
}

func displayStatus(status string) string {
	if status == "" {
		return "none"
	}
	return status
}
