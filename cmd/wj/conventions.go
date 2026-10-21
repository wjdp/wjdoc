package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

const conventions = `Filenames
  docs/NNN-Sentence-case-title.md
  NNN is the document's identity: never reused, never changed.
  Zero padding is presentation only. wj infers the width from docs/ and
  allocates at that width; new projects start at three digits.
  Slug is the title with spaces replaced by hyphens, in sentence case: first
  letter uppercase, the rest as written, so proper nouns keep their capitals
  (002-Prisma-to-Drizzle.md). No type prefix in the filename.
  000 (or 00) is the about-doc, type reference, and holds project specifics.

Frontmatter
  YAML between --- fences, the first thing in the file.
  type    required. reference (evergreen), review (point-in-time assessment),
          task (unit of work adding something new), bug (unit of work fixing
          existing behaviour).
  status  required for task, bug and review, absent for reference. todo
          (immediate), planned (longer term), in-progress, blocked, open
          (reviews only), shelved (parked indefinitely, tasks and bugs only),
          done, rejected (decided against, tasks and bugs only), superseded.
  Unknown type or status values warn, they do not error: the vocabulary grows.
  No created, updated or author fields: git already knows.

Body
  Title is the first "# " heading, falling back to the slug.
  Links between docs are relative markdown links to the filename. Links from
  code and agent files use docs/NNN-slug.md.
`

func conventionsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "conventions",
		Short: "Print the docs/ convention",
		Long: `Print the filename, frontmatter and body rules for docs/, as plain text.

Read this before creating or editing a document. The output is the same in
every mode and needs no project: it does not read docs/ and does not colour.

Examples:
  wj conventions           the rules
  wj conventions | head    same text, unchanged by piping`,
		Args: cobra.NoArgs,
		// Runs anywhere: no project root is needed, so skip the root lookup.
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		RunE: func(command *cobra.Command, _ []string) error {
			fmt.Fprint(command.OutOrStdout(), conventions)
			return nil
		},
	}
}
