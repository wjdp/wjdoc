package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wjdp/wjdoc/internal/docs"
	"github.com/wjdp/wjdoc/internal/move"
	"github.com/wjdp/wjdoc/internal/render"
)

// topFolder is how --to names the top level of docs/.
const topFolder = "top"

func moveCommand(shared *app) *cobra.Command {
	var to string
	var all, dryRun bool

	command := &cobra.Command{
		Use:   "move <ref>... [--to ref|task|top]",
		Short: "Move documents between the type folders and fix every link",
		Long: `Move documents into docs/ref/ or docs/task/, or back to the top of docs/, and
rewrite every link and reference to them.

With no --to, each document goes where its type says: reference to docs/ref/,
every other type to docs/task/, and the about-doc to the top of docs/. A project
becomes foldered when docs/ref/ or docs/task/ exists, so moving a document into
one is also how a project opts in.

--all moves every document that is not where it belongs, which migrates a flat
project in one step. With --to, --all moves every document but the about-doc.

Links between docs are recomputed from where each file will be, both into a
moved document and out of it, keeping anchors, titles and a leading ./.
docs/-prefixed paths are rewritten in every text file in the repository.
Documents move with git mv in a git repository. Nothing is committed: review the
working tree, then commit.

Examples:
  wj move --all --dry-run    list the moves and edits, change nothing
  wj move --all              migrate a flat project
  wj move 12                 move 12 to the folder that suits its type
  wj move 12 --to top        move 12 back to the top of docs/`,
		RunE: func(command *cobra.Command, args []string) error {
			if all == (len(args) > 0) {
				return fmt.Errorf("name the documents to move, or pass --all")
			}
			folder, err := moveFolder(to)
			if err != nil {
				return err
			}

			documents, err := docs.Load(shared.root)
			if err != nil {
				return err
			}

			var selected []docs.Document
			if all {
				for _, document := range documents {
					if document.Number >= 0 && !(to != "" && document.IsAboutDoc()) {
						selected = append(selected, document)
					}
				}
			} else {
				for _, ref := range args {
					document, err := docs.Resolve(documents, ref)
					if err != nil {
						return err
					}
					selected = append(selected, document)
				}
			}

			moves := map[string]string{}
			for _, document := range selected {
				destination := folder
				if to == "" {
					destination = move.Belongs(document)
				}
				if target := move.Destination(document, destination); target != document.Path {
					moves[document.Path] = target
				}
			}

			plan, err := move.New(shared.root, documents, moves)
			if err != nil {
				return err
			}
			return reportAndApply(command, shared, plan, dryRun)
		},
	}

	command.Flags().StringVar(&to, "to", "", "destination: `ref`, task or top, instead of the folder that suits each type")
	command.Flags().BoolVar(&all, "all", false, "move every document that is not where it belongs")
	command.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "print the plan without changing anything")
	return command
}

func moveFolder(to string) (string, error) {
	switch to {
	case "", topFolder:
		return move.Top, nil
	case docs.RefFolder, docs.TaskFolder:
		return to, nil
	}
	return "", fmt.Errorf("--to %q: want %s, %s or %s", to, docs.RefFolder, docs.TaskFolder, topFolder)
}

func reportAndApply(command *cobra.Command, shared *app, plan move.Plan, dryRun bool) error {
	out := command.OutOrStdout()
	if shared.mode() == render.JSON {
		if err := writePlanJSON(command, plan.Plan); err != nil {
			return err
		}
	} else {
		fmt.Fprint(out, plan.String())
	}

	if dryRun || plan.Empty() {
		return nil
	}
	if err := move.Apply(shared.root, plan); err != nil {
		return err
	}
	if shared.mode() != render.JSON {
		fmt.Fprintf(out, "moved %d files, edited %d references; review and commit\n", len(plan.Renames), len(plan.Edits))
	}
	return nil
}
