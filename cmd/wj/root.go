package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/wjdp/wjdoc/internal/docs"
	"github.com/wjdp/wjdoc/internal/render"
	"golang.org/x/term"
)

var version = "dev"

// defaultWidth is the table width assumed when stdout is not a terminal.
const defaultWidth = 80

// app holds the persistent flags and the project root resolved from them.
type app struct {
	plain     bool
	json      bool
	directory string
	root      string
}

func rootCommand() *cobra.Command {
	shared := &app{}

	root := &cobra.Command{
		Use:   "wj",
		Short: "Read, query and check the docs/ folder of a project",
		Long: `wj manages the numbered docs/ convention: reference, review and task documents with YAML frontmatter.
Run ` + "`wj conventions`" + ` for the rules.

The project root is the nearest directory at or above the working directory that
contains docs/, or failing that the nearest git repository.

Examples:
  wj list                       table of every doc in ./docs
  wj -C ~/local/grate list      the same for another project
  wj show 7 --body              print doc 7 without its frontmatter`,
		Version:           version,
		SilenceUsage:      true,
		SilenceErrors:     true,
		PersistentPreRunE: shared.resolveRoot,
	}

	root.PersistentFlags().BoolVar(&shared.plain, "plain", false, "tab-separated output with no colour or decoration")
	root.PersistentFlags().BoolVar(&shared.json, "json", false, "JSON output with stable field names")
	root.PersistentFlags().StringVarP(&shared.directory, "directory", "C", "", "run as if wj was started in `dir`")

	root.AddCommand(listCommand(shared), showCommand(shared), conventionsCommand())
	return root
}

func (a *app) resolveRoot(*cobra.Command, []string) error {
	start := a.directory
	if start == "" {
		working, err := os.Getwd()
		if err != nil {
			return err
		}
		start = working
	}

	root, err := docs.FindRoot(start)
	if err != nil {
		return err
	}
	a.root = root
	return nil
}

func (a *app) mode() render.Mode {
	return render.DetectMode(os.Stdout, a.plain, a.json)
}

// terminalWidth is the width of stdout, or defaultWidth when it is not a terminal.
func terminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return defaultWidth
	}
	return width
}
