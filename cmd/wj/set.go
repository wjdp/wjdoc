package main

import (
	"fmt"
	"os"
	"path"

	"github.com/spf13/cobra"
	"github.com/wjdp/wjdoc/internal/docs"
	"github.com/wjdp/wjdoc/internal/move"
	"github.com/wjdp/wjdoc/internal/render"
)

func setCommand(shared *app) *cobra.Command {
	var unset, moveFile bool

	command := &cobra.Command{
		Use:   "set <ref> <key> [value]",
		Short: "Edit one frontmatter key in place",
		Long: `Set or remove one frontmatter key on one document.

<ref> is resolved the same way as for wj show: an integer, a padded number, a
filename or path, or a fragment of the slug or title.

Only the key's own line is rewritten. Other keys, their order, comments, line
endings and the body are preserved byte for byte. A key that is not there yet is
appended to the block, and a file without frontmatter gains a block.

Examples:
  wj set 7 status done
  wj set sync status in-progress
  wj set 7 status --unset      drop the key entirely
  wj set 7 type reference --move

In a foldered project, a type change can leave a document in the wrong folder.
--move then moves it with wj move, rewriting its links; without it, set says
which wj move would fix it.`,
		Args: func(command *cobra.Command, args []string) error {
			if unset {
				return cobra.ExactArgs(2)(command, args)
			}
			return cobra.ExactArgs(3)(command, args)
		},
		RunE: func(command *cobra.Command, args []string) error {
			documents, err := docs.Load(shared.root)
			if err != nil {
				return err
			}

			document, err := docs.Resolve(documents, args[0])
			if err != nil {
				return err
			}

			key := args[1]
			value := ""
			if !unset {
				value = args[2]
			}

			source, err := os.ReadFile(document.AbsPath)
			if err != nil {
				return err
			}

			var edited []byte
			if unset {
				edited, err = docs.UnsetField(source, key)
			} else {
				edited, err = docs.SetField(source, key, value)
			}
			if err != nil {
				return err
			}

			info, err := os.Stat(document.AbsPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(document.AbsPath, edited, info.Mode().Perm()); err != nil {
				return err
			}

			if key == "type" && docs.Foldered(shared.root) {
				if err := followType(command, shared, document.Path, moveFile); err != nil {
					return err
				}
			}

			if shared.mode() == render.Pretty {
				if unset {
					fmt.Fprintf(command.OutOrStdout(), "%s: %s unset\n", document.Path, key)
				} else {
					fmt.Fprintf(command.OutOrStdout(), "%s: %s = %s\n", document.Path, key, value)
				}
			}
			return nil
		},
	}

	command.Flags().BoolVar(&unset, "unset", false, "remove the key instead of setting it")
	command.Flags().BoolVar(&moveFile, "move", false, "after changing type, move the document to the folder that suits it")
	return command
}

// followType moves a document whose type no longer suits its folder, or with
// moveFile unset says how to.
func followType(command *cobra.Command, shared *app, documentPath string, moveFile bool) error {
	documents, err := docs.Load(shared.root)
	if err != nil {
		return err
	}
	document, err := docs.Resolve(documents, documentPath)
	if err != nil {
		return err
	}
	target := move.Destination(document, move.Belongs(document))
	if document.Number < 0 || target == document.Path || document.Folder() == "" && !moveFile {
		return nil
	}
	if !moveFile {
		fmt.Fprintf(command.ErrOrStderr(), "%s now belongs in %s: run `wj move %d`\n", document.Path, path.Dir(target)+"/", document.Number)
		return nil
	}

	plan, err := move.New(shared.root, documents, map[string]string{document.Path: target})
	if err != nil {
		return err
	}
	return reportAndApply(command, shared, plan, false)
}
