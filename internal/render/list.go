package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/wjdp/wjdoc/internal/docs"
)

// titleColumnWidth is the terminal width at and above which the Title
// column is shown in the pretty table (docs/001-Design.md "Output").
const titleColumnWidth = 120

// List writes docs in the given mode. width is the terminal width the
// caller measured; it only affects the Pretty table's Title column.
func List(w io.Writer, docs []docs.Document, mode Mode, width int) {
	switch mode {
	case JSON:
		listJSON(w, docs)
	case Plain:
		listPlain(w, docs)
	default:
		listPretty(w, docs, width)
	}
}

func listPlain(w io.Writer, docs []docs.Document) {
	for _, doc := range docs {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", doc.Path, doc.Type, doc.Status, doc.Title, isoDate(doc.Modified))
	}
}

// isoDate is the date part of a modification time, empty when git knows none.
func isoDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// RelativeAge is a compact age for a modification time: "today", "3d", "2w",
// "5mo", "1y". A zero time has no age.
func RelativeAge(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	days := int(time.Since(t).Hours() / 24)
	switch {
	case days <= 0:
		return "today"
	case days < 7:
		return fmt.Sprintf("%dd", days)
	case days < 30:
		return fmt.Sprintf("%dw", days/7)
	case days < 365:
		return fmt.Sprintf("%dmo", days/30)
	default:
		return fmt.Sprintf("%dy", days/365)
	}
}

func listJSON(w io.Writer, docs []docs.Document) {
	rows := make([]jsonDoc, len(docs))
	for i, doc := range docs {
		rows[i] = toJSONDoc(doc)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(rows)
}

func listPretty(w io.Writer, list []docs.Document, width int) {
	showTitle := width >= titleColumnWidth

	headers := []string{"Document", "Type", "Status", "Modified"}
	if showTitle {
		headers = append(headers, "Title")
	}

	// Column widths come from plain (unstyled) content only.
	plainCell := func(doc docs.Document, col int) string {
		switch col {
		case 0:
			return doc.Path
		case 1:
			return displayOrDash(doc.Type)
		case 2:
			return displayOrDash(doc.Status)
		case 3:
			return RelativeAge(doc.Modified)
		default:
			return doc.Title
		}
	}
	plainWidths := make([]int, len(headers))
	for i, h := range headers {
		plainWidths[i] = lipgloss.Width(h)
	}
	for _, doc := range list {
		for col := range headers {
			if width := lipgloss.Width(plainCell(doc, col)); width > plainWidths[col] {
				plainWidths[col] = width
			}
		}
	}

	rows := make([][]string, 0, len(list))
	for _, doc := range list {
		cells := []string{
			hyperlink(doc.AbsPath, pathStyle.Render(doc.Path)),
			StyleValue(doc.Type, typeStyles),
			StyleValue(doc.Status, statusStyles),
			faintStyle.Render(padLeft(RelativeAge(doc.Modified), plainWidths[3])),
		}
		if showTitle {
			cells = append(cells, titleStyle.Render(doc.Title))
		}
		rows = append(rows, cells)
	}

	var b strings.Builder
	for i, h := range headers {
		b.WriteString(headerStyle.Render(padRight(h, plainWidths[i])))
		if i < len(headers)-1 {
			b.WriteString(" ")
		}
	}
	b.WriteString("\n")

	for _, cells := range rows {
		for i, cell := range cells {
			b.WriteString(padRight(cell, plainWidths[i]))
			if i < len(cells)-1 {
				b.WriteString(" ")
			}
		}
		b.WriteString("\n")
	}
	fmt.Fprint(w, b.String())

	for _, doc := range list {
		if doc.Err != nil {
			fmt.Fprintln(w, errorStyle.Render(fmt.Sprintf("invalid frontmatter %s: %s", doc.Path, doc.Err)))
		}
	}
}

func displayOrDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}

// padRight pads to width by display width, so ANSI escapes never throw off
// column alignment.
func padRight(s string, width int) string {
	gap := width - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
}

// padLeft right-aligns s within width, by display width.
func padLeft(s string, width int) string {
	gap := width - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	return strings.Repeat(" ", gap) + s
}

// hyperlink wraps text in an OSC 8 escape sequence linking to target. It adds
// no visible width, so padding must be computed on the unwrapped text first.
func hyperlink(target, text string) string {
	return "\x1b]8;;file://" + target + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}
