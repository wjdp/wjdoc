package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/wjdp/wjdoc/internal/docs"
	"github.com/wjdp/wjdoc/internal/lint"
	"github.com/wjdp/wjdoc/internal/render"
)

var (
	lintErrorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	lintWarningStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
	lintSummaryStyle = lipgloss.NewStyle().Faint(true)
)

// errLintFailed exits non-zero without adding anything to the findings already printed.
var errLintFailed = errors.New("lint found errors")

type lintFinding struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

func lintCommand(shared *app) *cobra.Command {
	return &cobra.Command{
		Use:   "lint [paths...]",
		Short: "Check docs/ against the convention",
		Long: `Check every document in docs/ against the convention and report what is wrong.

Errors: missing or unparseable frontmatter, missing type, missing status on a
task or review, two documents sharing a number, a filename that is not
NNN-Sentence-case-title.md, and a relative link whose target does not exist.
Warnings: an unknown type or status, a status on a reference, a padding width
that differs from the rest of the project, and a link to a superseded document.

Exit status is 1 when there is at least one error and 0 otherwise, so warnings
never fail a run. Nothing is printed when a run is clean and output is piped.

Paths narrow the report without narrowing the checks: every rule still runs over
every document, because duplicate numbers and padding widths are properties of
the set, but only findings for the named documents are printed. A path may be
relative to the working directory or to the project root, or a bare filename.

Piped, or with --plain: one finding per line, "path:line: level: message", with
":line" omitted when the problem belongs to the file rather than a line.
On a terminal: the same lines with the level coloured, then a count.
With --json: an array of objects with path, line, level and message.

Examples:
  wj lint                            check the whole project
  wj lint docs/007-Sync.md           report only this document
  wj lint --json                     the same findings as JSON
  wj lint --plain && echo clean      check in a script`,
		RunE: func(command *cobra.Command, args []string) error {
			documents, err := docs.Load(shared.root)
			if err != nil {
				return err
			}

			findings := lint.Run(shared.root, documents)
			failed := lint.HasErrors(findings)
			if len(args) > 0 {
				findings = selectPaths(findings, shared.root, args)
				failed = lint.HasErrors(findings)
			}

			if err := writeFindings(command.OutOrStdout(), findings, shared.mode()); err != nil {
				return err
			}
			if failed {
				return errLintFailed
			}
			return nil
		},
	}
}

func writeFindings(out io.Writer, findings []lint.Finding, mode render.Mode) error {
	if mode == render.JSON {
		rows := make([]lintFinding, 0, len(findings))
		for _, finding := range findings {
			rows = append(rows, lintFinding{
				Path:    finding.Path,
				Line:    finding.Line,
				Level:   finding.Level.String(),
				Message: finding.Message,
			})
		}
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(rows)
	}

	errorCount, warningCount := 0, 0
	for _, finding := range findings {
		level := finding.Level.String()
		if finding.Level == lint.Error {
			errorCount++
			if mode == render.Pretty {
				level = lintErrorStyle.Render(level)
			}
		} else {
			warningCount++
			if mode == render.Pretty {
				level = lintWarningStyle.Render(level)
			}
		}
		fmt.Fprintf(out, "%s: %s: %s\n", location(finding), level, finding.Message)
	}

	if mode != render.Pretty {
		return nil
	}
	if len(findings) == 0 {
		fmt.Fprintln(out, "clean")
		return nil
	}
	fmt.Fprintln(out, lintSummaryStyle.Render(fmt.Sprintf("%d %s, %d %s",
		errorCount, plural(errorCount, "error", "errors"),
		warningCount, plural(warningCount, "warning", "warnings"))))
	return nil
}

func location(finding lint.Finding) string {
	if finding.Line == 0 {
		return finding.Path
	}
	return fmt.Sprintf("%s:%d", finding.Path, finding.Line)
}

// selectPaths keeps the findings for the named documents. A name may be a path
// relative to the working directory or to the project root, or a bare filename.
func selectPaths(findings []lint.Finding, root string, names []string) []lint.Finding {
	wanted := map[string]bool{}
	for _, name := range names {
		wanted[filepath.ToSlash(filepath.Clean(name))] = true
		if absolute, err := filepath.Abs(name); err == nil {
			if relative, err := filepath.Rel(root, absolute); err == nil {
				wanted[filepath.ToSlash(relative)] = true
			}
		}
	}

	kept := make([]lint.Finding, 0, len(findings))
	for _, finding := range findings {
		if wanted[finding.Path] || wanted[filepath.ToSlash(filepath.Base(finding.Path))] {
			kept = append(kept, finding)
		}
	}
	return kept
}
