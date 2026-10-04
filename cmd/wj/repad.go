package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wjdp/wjdoc/internal/docs"
	"github.com/wjdp/wjdoc/internal/render"
	"github.com/wjdp/wjdoc/internal/repad"
)

func repadCommand(shared *app) *cobra.Command {
	var width int
	var dryRun bool

	command := &cobra.Command{
		Use:   "repad --width N",
		Short: "Change the padding width of every document and fix the references",
		Long: `Re-pad every numbered document in docs/ to N digits and rewrite every reference
to the old filenames.

A document's identity is its integer; the zero padding is presentation only. So
this is the one sanctioned mass rename: everything keeps its number and only the
filenames change. wj lint warns when padding widths are mixed, and this is the
command that warning leads to. Run it when a width is exhausted, which is also
what wj new says when it refuses to allocate.

References are rewritten in every text file in the repository: relative links
between docs, ` + "`docs/NNN-slug.md`" + ` mentions in code and agent files, and the
%20-escaped spellings of names containing spaces. Documents move with git mv in
a git repository. Nothing is committed: review the working tree, then commit.

Examples:
  wj repad --width 3 --dry-run   list the renames and edits, change nothing
  wj repad --width 3             make the change
  wj repad --width 3 --json`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			documents, err := docs.Load(shared.root)
			if err != nil {
				return err
			}

			plan, err := repad.New(shared.root, documents, width)
			if err != nil {
				return err
			}

			out := command.OutOrStdout()
			if shared.mode() == render.JSON {
				if err := writePlanJSON(command, plan); err != nil {
					return err
				}
			} else {
				fmt.Fprint(out, plan.String())
			}

			if dryRun {
				return nil
			}
			if err := repad.Apply(shared.root, plan); err != nil {
				return err
			}
			if shared.mode() != render.JSON {
				fmt.Fprintf(out, "renamed %d files, edited %d references; review and commit\n", len(plan.Renames), len(plan.Edits))
			}
			return nil
		},
	}

	command.Flags().IntVar(&width, "width", 0, "zero-padding `width` in digits")
	command.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "print the plan without changing anything")
	command.MarkFlagRequired("width")
	return command
}

type renameJSON struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type editJSON struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	From string `json:"from"`
	To   string `json:"to"`
}

func writePlanJSON(command *cobra.Command, plan repad.Plan) error {
	renames := make([]renameJSON, 0, len(plan.Renames))
	for _, rename := range plan.Renames {
		renames = append(renames, renameJSON{rename.From, rename.To})
	}

	edits := make([]editJSON, 0, len(plan.Edits))
	for _, edit := range plan.Edits {
		edits = append(edits, editJSON{edit.Path, edit.Line, edit.From, edit.To})
	}

	encoder := json.NewEncoder(command.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(struct {
		Renames []renameJSON `json:"renames"`
		Edits   []editJSON   `json:"edits"`
	}{renames, edits})
}
