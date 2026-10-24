package main

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/wjdp/wjdoc/internal/docs"
	"github.com/wjdp/wjdoc/internal/render"
)

func tagCommand(shared *app) *cobra.Command {
	var remove bool

	command := &cobra.Command{
		Use:   "tag <ref> <tag>...",
		Short: "Add or remove tags on one document",
		Long: `Add tags to one document, or remove them with --remove.

<ref> is resolved the same way as for wj show. A tag is a lowercase name or
namespace:value made of letters, digits and hyphens, e.g. area:engine.

The tags key is rewritten as one flow-style line, tags: [area:engine, reach:app],
replacing a block-style list if there was one. Every other byte of the file is
preserved. Tags keep the order they were added in; adding one already present
or removing one that is absent changes nothing. Removing the last tag drops the
key.

Run wj tags to see the tags already in use before inventing a new one.

Examples:
  wj tag 7 area:engine reach:app
  wj tag 7 --remove reach:app`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			documents, err := docs.Load(shared.root)
			if err != nil {
				return err
			}
			document, err := docs.Resolve(documents, args[0])
			if err != nil {
				return err
			}
			if _, present := document.Frontmatter["tags"]; present && document.Tags == nil {
				return fmt.Errorf("%s: tags is not a list of strings, fix it by hand first", document.Path)
			}

			changes := make([]string, 0, len(args)-1)
			for _, tag := range args[1:] {
				tag = strings.ToLower(strings.TrimSpace(tag))
				if !docs.TagPattern.MatchString(tag) {
					return fmt.Errorf("tag %q is not a lowercase name or namespace:value", tag)
				}
				changes = append(changes, tag)
			}

			tags := slices.Clone(document.Tags)
			for _, tag := range changes {
				switch {
				case remove:
					tags = slices.DeleteFunc(tags, func(existing string) bool { return existing == tag })
				case !slices.Contains(tags, tag):
					tags = append(tags, tag)
				}
			}

			source, err := os.ReadFile(document.AbsPath)
			if err != nil {
				return err
			}
			edited, err := docs.SetList(source, "tags", tags)
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

			if shared.mode() == render.Pretty {
				fmt.Fprintf(command.OutOrStdout(), "%s: tags = [%s]\n", document.Path, strings.Join(tags, ", "))
			}
			return nil
		},
	}

	command.Flags().BoolVar(&remove, "remove", false, "remove the tags instead of adding them")
	return command
}

// tagCount is one row of wj tags, and its --json shape.
type tagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

var tagsTitleStyle = lipgloss.NewStyle().Bold(true)

func tagsCommand(shared *app) *cobra.Command {
	var all, active bool

	command := &cobra.Command{
		Use:   "tags",
		Short: "List the tags in use and how often",
		Long: `List every tag used in docs/ with the number of documents carrying it.

Tags are grouped by namespace (the part before ":"), namespaces alphabetically,
with the most used tag first within each and ties alphabetical. Tags without a
namespace come last. --active counts only documents that are not shelved, done,
rejected or superseded. --all counts across every sibling project, as for
wj list --all.

On a terminal: a bold "tags · N" line, then one aligned row per tag.
Piped, or with --plain: one line per tag, tab-separated: count, tag.
With --json: an array of {tag, count} objects.

Examples:
  wj tags                  the tag vocabulary of this project
  wj tags --all --active   what is being worked on everywhere, by tag`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			documents, _, err := listDocuments(shared, all)
			if err != nil {
				return err
			}
			if active {
				documents = activeOnly(documents)
			}
			counts := countTags(documents)

			out := command.OutOrStdout()
			switch shared.mode() {
			case render.JSON:
				return writeIndentedJSON(out, counts)
			case render.Plain:
				for _, row := range counts {
					fmt.Fprintf(out, "%d\t%s\n", row.Count, row.Tag)
				}
			default:
				fmt.Fprintln(out, tagsTitleStyle.Render(fmt.Sprintf("tags · %d", len(counts))))
				width := 0
				for _, row := range counts {
					width = max(width, len(fmt.Sprint(row.Count)))
				}
				for _, row := range counts {
					fmt.Fprintf(out, "%*d  %s\n", width, row.Count, row.Tag)
				}
			}
			return nil
		},
	}

	command.Flags().BoolVar(&all, "all", false, "count across every sibling project, not just this one")
	command.Flags().BoolVarP(&active, "active", "a", false, "count only documents that are not shelved, done, rejected or superseded")
	return command
}

func countTags(documents []docs.Document) []tagCount {
	counts := map[string]int{}
	for _, document := range documents {
		for _, tag := range document.Tags {
			counts[tag]++
		}
	}

	rows := make([]tagCount, 0, len(counts))
	for tag, count := range counts {
		rows = append(rows, tagCount{tag, count})
	}
	sort.Slice(rows, func(i, j int) bool {
		left, right := namespace(rows[i].Tag), namespace(rows[j].Tag)
		if (left == "") != (right == "") {
			return right == ""
		}
		if left != right {
			return left < right
		}
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].Tag < rows[j].Tag
	})
	return rows
}

func namespace(tag string) string {
	name, _, found := strings.Cut(tag, ":")
	if !found {
		return ""
	}
	return name
}
