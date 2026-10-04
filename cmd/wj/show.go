package main

import (
	"github.com/spf13/cobra"
	"github.com/wjdp/wjdoc/internal/docs"
	"github.com/wjdp/wjdoc/internal/render"
)

func showCommand(shared *app) *cobra.Command {
	var bodyOnly bool

	command := &cobra.Command{
		Use:     "show <ref>",
		Aliases: []string{"cat"},
		Short:   "Print one document",
		Long: `Print one document from docs/.

<ref> is an integer (7), a padded number (007), a filename or path, or a
fragment of the slug or title. Exact matches win over fragments; a fragment
matching more than one document is an error listing the candidates, and exits 1.

On a terminal: a bold header line of path, type and status, then the file.
Piped, or with --plain: the file's bytes, frontmatter included, and nothing else.
With --body: the markdown after the frontmatter fence only.
With --json: one object with number, path, type, status, title, modified, error
and body; --body is ignored because the body is always a separate field.

Examples:
  wj show 7                the file with number 7, frontmatter and all
  wj show sync --body      match by slug fragment, print the markdown only
  wj show 7 --json         the same document as JSON`,
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

			render.Show(command.OutOrStdout(), document, shared.mode(), bodyOnly)
			return nil
		},
	}

	command.Flags().BoolVar(&bodyOnly, "body", false, "print the body only, without the frontmatter fence")
	return command
}
