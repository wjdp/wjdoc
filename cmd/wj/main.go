package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	if err := rootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "wj:", err)
		os.Exit(1)
	}
}

func rootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "wj",
		Short:         "Read, query and check the docs/ folder of a project",
		Long:          "wj manages the numbered docs/ convention: reference, review and task documents with YAML frontmatter.\nRun `wj conventions` for the rules.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	return root
}
