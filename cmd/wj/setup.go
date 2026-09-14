package main

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

//go:embed skill/SKILL.md
var skillContent string

func setupCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Install the wj-docs Claude Code skill",
		Long: `Write the wj-docs skill to ~/.claude/skills/wj-docs/SKILL.md.

Describes the docs/ workflow (wj next, wj new, wj set status, wj lint) so an
agent discovers it without being told in prose. Safe to run more than once:
an up to date install is left alone, a stale one is overwritten, and anything
else at that path is left untouched and reported.

Examples:
  wj setup    install or update the skill`,
		Args: cobra.NoArgs,
		// Runs anywhere: installs into the home directory, not a project.
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		RunE: func(command *cobra.Command, _ []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}

			skillDir := filepath.Join(home, ".claude", "skills", "wj-docs")
			target := filepath.Join(skillDir, "SKILL.md")
			out := command.OutOrStdout()

			if err := os.MkdirAll(skillDir, 0o755); err != nil {
				return fmt.Errorf("create %s: %w", skillDir, err)
			}

			existing, err := os.ReadFile(target)
			switch {
			case errors.Is(err, os.ErrNotExist):
				if err := os.WriteFile(target, []byte(skillContent), 0o644); err != nil {
					return fmt.Errorf("write %s: %w", target, err)
				}
				fmt.Fprintf(out, "installed %s\n", target)
			case err != nil:
				return fmt.Errorf("read %s: %w", target, err)
			case string(existing) == skillContent:
				fmt.Fprintf(out, "%s exists\n", target)
			default:
				if err := os.WriteFile(target, []byte(skillContent), 0o644); err != nil {
					return fmt.Errorf("write %s: %w", target, err)
				}
				fmt.Fprintf(out, "updated %s\n", target)
			}
			return nil
		},
	}
}
