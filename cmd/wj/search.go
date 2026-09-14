package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wjdp/wjdoc/internal/docs"
	"github.com/wjdp/wjdoc/internal/render"
)

// searchMatch is one ripgrep hit, and the --json shape for wj search.
type searchMatch struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

func searchCommand(shared *app) *cobra.Command {
	var types, statuses []string
	var useRegex bool

	command := &cobra.Command{
		Use:   "search <term>",
		Short: "Search docs/ with ripgrep",
		Long: `Search the documents in docs/ for term, using ripgrep (rg), which must be
on PATH.

term is a fixed string by default; pass --regex to treat it as a ripgrep
regular expression instead. --type and --status filter which documents are
searched first, same as wj list.

On a terminal: rg's own coloured output, path in bold, matches highlighted.
Piped, or with --plain: rg's plain "path:line:text" output, one per match,
paths relative to the project root.
With --json: an array of {path, line, text} objects.

Exits 1 when there are no matches, same as rg.

Examples:
  wj search "sync robustness"            plain-text search
  wj search '^## ' --regex               a regular expression
  wj search TODO --type task --status todo   only todo tasks`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if _, err := exec.LookPath("rg"); err != nil {
				return errors.New("wj search needs ripgrep (rg) on PATH")
			}

			documents, err := docs.Load(shared.root)
			if err != nil {
				return err
			}
			documents = docs.Filter(documents, types, statuses)

			paths := make([]string, 0, len(documents))
			for _, document := range documents {
				paths = append(paths, document.Path)
			}
			if len(paths) == 0 {
				return errors.New("no matches")
			}

			mode := shared.mode()
			rgArgs := []string{"--line-number", "--no-heading"}
			if mode == render.Pretty {
				rgArgs = append(rgArgs, "--color", "always", "--colors", "path:style:bold")
			} else {
				rgArgs = append(rgArgs, "--color", "never")
			}
			if !useRegex {
				rgArgs = append(rgArgs, "--fixed-strings")
			}
			rgArgs = append(rgArgs, args[0])
			rgArgs = append(rgArgs, paths...)

			rg := exec.Command("rg", rgArgs...)
			rg.Dir = shared.root
			output, runErr := rg.Output()

			exitErr := &exec.ExitError{}
			if errors.As(runErr, &exitErr) {
				if exitErr.ExitCode() == 1 {
					return errors.New("no matches")
				}
				return fmt.Errorf("rg: %s", strings.TrimSpace(string(exitErr.Stderr)))
			}
			if runErr != nil {
				return runErr
			}

			if mode == render.JSON {
				return writeSearchJSON(command.OutOrStdout(), output)
			}
			fmt.Fprint(command.OutOrStdout(), string(output))
			return nil
		},
	}

	command.Flags().StringSliceVar(&types, "type", nil, "search only these types (comma-separated, repeatable)")
	command.Flags().StringSliceVar(&statuses, "status", nil, "search only these statuses (comma-separated, repeatable)")
	command.Flags().BoolVar(&useRegex, "regex", false, "treat term as a ripgrep regular expression")
	return command
}

// writeSearchJSON re-runs ripgrep's plain "path:line:text" output as an array
// of {path, line, text} objects. Called only when mode is JSON, so the output
// passed in was produced with --color never.
func writeSearchJSON(w interface{ Write([]byte) (int, error) }, output []byte) error {
	matches := []searchMatch{}
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, ":", 3)
		if len(fields) != 3 {
			continue
		}
		lineNumber, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		matches = append(matches, searchMatch{Path: fields[0], Line: lineNumber, Text: fields[2]})
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(matches)
}
