package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/wjdp/wjdoc/internal/docs"
	"github.com/wjdp/wjdoc/internal/links"
	"github.com/wjdp/wjdoc/internal/project"
	"github.com/wjdp/wjdoc/internal/render"
)

var (
	linkPathStyle    = lipgloss.NewStyle().Bold(true)
	linkMissingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	linkLineStyle    = lipgloss.NewStyle().Faint(true)
)

type outboundLink struct {
	Target string `json:"target"`
	Line   int    `json:"line"`
	Exists bool   `json:"exists"`
}

type inboundLink struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}

func linksCommand(shared *app) *cobra.Command {
	return &cobra.Command{
		Use:   "links <ref>",
		Short: "List the files a document links to",
		Long: `List the relative markdown links out of one document, resolved to repository paths.

<ref> is an integer, a padded number, a path or filename, or a fragment of the
slug or title. Links inside fenced code blocks and code spans are ignored, as
are absolute URLs; a target that does not exist is marked missing.

Piped, or with --plain: one repository path per line, with " missing" appended
when the target is not there. On a terminal: the same paths in bold, with the
line they appear on. With --json: an array of objects with target, line and
exists.

Examples:
  wj links 7                 what doc 7 points at
  wj links 7 --json          the same as JSON
  wj links Design --plain    by title fragment`,
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

			offset := project.BodyOffset(document)
			found := make([]outboundLink, 0)
			for _, link := range links.Extract(document.Body) {
				target := links.ResolveTarget(document.Path, link.Target)
				found = append(found, outboundLink{
					Target: target,
					Line:   link.Line + offset,
					Exists: targetExists(filepath.Join(shared.root, filepath.FromSlash(target))),
				})
			}
			return writeOutboundLinks(command.OutOrStdout(), found, shared.mode())
		},
	}
}

func backlinksCommand(shared *app) *cobra.Command {
	return &cobra.Command{
		Use:   "backlinks <ref>",
		Short: "List the files that link to a document",
		Long: `List everywhere a document is referred to, as path:line.

Other documents count when they link to it relatively; every other file in the
repository counts when it mentions the document's path or filename. The
repository scan uses ripgrep and is skipped when ripgrep is not installed, so
only document-to-document links are reported then.

Piped, or with --plain: one "path:line" per line. On a terminal: the same with
the path in bold. With --json: an array of objects with path and line.

Examples:
  wj backlinks 1             who points at doc 1
  wj backlinks 1 --json      the same as JSON`,
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

			references, err := project.Backlinks(shared.root, document, documents)
			if err != nil {
				return err
			}
			return writeBacklinks(command.OutOrStdout(), references, shared.mode())
		},
	}
}

func writeOutboundLinks(out io.Writer, found []outboundLink, mode render.Mode) error {
	if mode == render.JSON {
		return writeLinkJSON(out, found)
	}
	for _, link := range found {
		switch mode {
		case render.Pretty:
			line := linkPathStyle.Render(link.Target) + " " + linkLineStyle.Render(fmt.Sprintf("line %d", link.Line))
			if !link.Exists {
				line += " " + linkMissingStyle.Render("missing")
			}
			fmt.Fprintln(out, line)
		default:
			if link.Exists {
				fmt.Fprintln(out, link.Target)
			} else {
				fmt.Fprintln(out, link.Target+" missing")
			}
		}
	}
	return nil
}

func writeBacklinks(out io.Writer, references []project.Reference, mode render.Mode) error {
	if mode == render.JSON {
		rows := make([]inboundLink, 0, len(references))
		for _, reference := range references {
			rows = append(rows, inboundLink{Path: reference.Path, Line: reference.Line})
		}
		return writeLinkJSON(out, rows)
	}

	for _, reference := range references {
		path := reference.Path
		if mode == render.Pretty {
			path = linkPathStyle.Render(path)
		}
		fmt.Fprintf(out, "%s:%d\n", path, reference.Line)
	}
	return nil
}

func writeLinkJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func targetExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
