package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/wjdp/wjdoc/internal/docs"
	"github.com/wjdp/wjdoc/internal/gitx"
	"github.com/wjdp/wjdoc/internal/project"
	"github.com/wjdp/wjdoc/internal/render"
)

func initCommand(shared *app) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Set a project up for wj",
		Long: `Create docs/ and its about-doc, point the project's agent file at wj and install
the pre-commit lint hook.

Run in an existing project: with no docs/ yet the git repository containing the
working directory is used, or the working directory itself, or the directory
given with -C.

Four steps, each safe to repeat: docs/ is created when missing; the about-doc
NNN-Docs.md is written when no document numbered 0 exists, at the project's
existing padding width; one line is appended to AGENTS.md, or CLAUDE.md when
only that exists, unless either file already carries it; and .git/hooks/pre-commit
is written to run wj lint over staged documents. A pre-commit hook wj did not
write is never touched: the lines to add are printed instead.

Each step prints what happened, as "created docs/000-Docs.md".

Examples:
  wj init                    set up the project here
  wj -C ~/local/grate init   set up another project`,
		Args: cobra.NoArgs,
		// Runs before docs/ exists, so resolve the root leniently.
		PersistentPreRunE: func(*cobra.Command, []string) error {
			root, err := initRoot(shared.directory)
			if err != nil {
				return err
			}
			shared.root = root
			return nil
		},
		RunE: func(command *cobra.Command, _ []string) error {
			report, err := project.Init(shared.root, filepath.Base(shared.root))
			if err != nil {
				return err
			}

			out := command.OutOrStdout()
			for _, action := range report {
				line := action.Done + " " + action.Path
				if action.Note != "" {
					line += " (" + action.Note + ")"
				}
				if shared.mode() == render.Pretty {
					line = createdStyle.Render(line)
				}
				fmt.Fprintln(out, line)
			}

			if report.ForeignHook() {
				fmt.Fprintf(out, "\nadd this to .git/hooks/pre-commit by hand:\n\n%s\n", project.LintHook)
			}
			return nil
		},
	}
}

// initRoot is the project root to set up: the nearest project at or above the
// starting directory, the git repository containing it, or the directory itself.
func initRoot(directory string) (string, error) {
	start := directory
	if start == "" {
		working, err := os.Getwd()
		if err != nil {
			return "", err
		}
		start = working
	}
	start, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}

	if root, err := docs.FindRoot(start); err == nil {
		return root, nil
	}
	if toplevel, err := gitx.Run(start, "rev-parse", "--show-toplevel"); err == nil && toplevel != "" {
		return toplevel, nil
	}
	return start, nil
}
