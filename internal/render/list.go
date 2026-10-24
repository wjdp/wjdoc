package render

import (
	"encoding/json"
	"fmt"
	"io"
	"path"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/wjdp/wjdoc/internal/docs"
)

const (
	// minTitleWidth is the narrowest Title column the table will ever print.
	minTitleWidth = 12
	// roomyTitleWidth is the width Title is given before optional columns stop
	// being dropped to make space for it.
	roomyTitleWidth = 30
)

// column identifies one table column.
type column int

const (
	colNumber column = iota
	colPath
	colStatus
	colPriority
	colTitle
	colTags
	colType
	colCreated
	colModified
)

// dropOrder is the order optional columns give up their space when the
// terminal is too narrow for a comfortable Title.
var dropOrder = []column{colType, colTags, colCreated, colModified, colPriority}

var headers = map[column]string{
	colNumber:   "#",
	colPath:     "Document",
	colStatus:   "Status",
	colPriority: "Pri",
	colTitle:    "Title",
	colTags:     "Tags",
	colType:     "Type",
	colCreated:  "Created",
	colModified: "Modified",
}

// Columns are the optional columns of the Pretty table.
type Columns struct {
	// Paths adds the repo-relative path as its own column.
	Paths bool
	// Tags adds the document's tags.
	Tags bool
}

// List writes docs in the given mode. width is the terminal width the caller
// measured; it and columns only affect the Pretty table.
func List(w io.Writer, list []docs.Document, mode Mode, width int, columns Columns) {
	switch mode {
	case JSON:
		listJSON(w, list)
	case Plain:
		listPlain(w, list)
	default:
		listPretty(w, list, width, columns)
	}
}

func listPlain(w io.Writer, list []docs.Document) {
	for _, doc := range list {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			doc.Path, doc.Type, doc.Status, doc.Title, isoDate(doc.Created), isoDate(doc.Modified),
			doc.Priority, strings.Join(doc.Tags, ","))
	}
}

// isoDate is the date part of a commit time, empty when git knows none.
func isoDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// RelativeAge is a compact age for a commit time: "today", "3d", "2w", "5mo",
// "1y". A zero time has no age.
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

func listJSON(w io.Writer, list []docs.Document) {
	rows := make([]jsonDoc, len(list))
	for i, doc := range list {
		rows[i] = toJSONDoc(doc)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(rows)
}

// numberCell is the padded document number, or the filename for a file that
// does not follow the numbering convention.
func numberCell(doc docs.Document) string {
	if doc.Number < 0 {
		return path.Base(doc.Path)
	}
	width := doc.Width
	if width < 1 {
		width = 1
	}
	return fmt.Sprintf("%0*d", width, doc.Number)
}

func plainCell(doc docs.Document, col column) string {
	switch col {
	case colNumber:
		return numberCell(doc)
	case colPath:
		return doc.Path
	case colStatus:
		return doc.Status
	case colPriority:
		return doc.Priority
	case colTitle:
		return titleText(doc)
	case colTags:
		return strings.Join(doc.Tags, ", ")
	case colType:
		return doc.Type
	case colCreated:
		return RelativeAge(doc.Created)
	default:
		return RelativeAge(doc.Modified)
	}
}

// titleText is the document's title, or its parse error when it has one.
func titleText(doc docs.Document) string {
	if doc.Err != nil {
		return doc.Err.Error()
	}
	return doc.Title
}

func listPretty(w io.Writer, list []docs.Document, width int, optional Columns) {
	columns := []column{colNumber}
	if optional.Paths {
		columns = append(columns, colPath)
	}
	columns = append(columns, colStatus)
	if anyPriority(list) {
		columns = append(columns, colPriority)
	}
	columns = append(columns, colTitle)
	if optional.Tags {
		columns = append(columns, colTags)
	}
	columns = append(columns, colType, colCreated, colModified)

	widths := map[column]int{}
	for _, col := range columns {
		widths[col] = lipgloss.Width(headers[col])
		for _, doc := range list {
			if cell := lipgloss.Width(plainCell(doc, col)); cell > widths[col] {
				widths[col] = cell
			}
		}
	}

	for _, droppable := range dropOrder {
		if titleSpace(columns, widths, width) >= roomyTitleWidth {
			break
		}
		columns = without(columns, droppable)
	}
	widths[colTitle] = min(widths[colTitle], max(titleSpace(columns, widths, width), minTitleWidth))
	if slices.Contains(columns, colTags) {
		widths[colTags] = min(widths[colTags], max(widths[colTitle], minTitleWidth))
	}

	var b strings.Builder
	for index, col := range columns {
		b.WriteString(headerStyle.Render(padded(headers[col], widths[col], col)))
		if index < len(columns)-1 {
			b.WriteString(" ")
		}
	}
	b.WriteString("\n")

	for _, doc := range list {
		var row strings.Builder
		for index, col := range columns {
			row.WriteString(styledCell(doc, col, widths[col]))
			if index < len(columns)-1 {
				row.WriteString(" ")
			}
		}
		b.WriteString(strings.TrimRight(row.String(), " "))
		b.WriteString("\n")
	}
	b.WriteString(faintStyle.Render(countsLine(list)))
	b.WriteString("\n")
	fmt.Fprint(w, b.String())

	for _, doc := range list {
		if doc.Err != nil {
			fmt.Fprintln(w, errorStyle.Render(fmt.Sprintf("invalid frontmatter %s: %s", doc.Path, doc.Err)))
		}
	}
}

func anyPriority(list []docs.Document) bool {
	for _, doc := range list {
		if doc.Priority != "" {
			return true
		}
	}
	return false
}

// titleSpace is the room left for Title once every other shown column has
// taken its width, separators included.
func titleSpace(columns []column, widths map[column]int, width int) int {
	space := width - (len(columns) - 1)
	for _, col := range columns {
		if col != colTitle {
			space -= widths[col]
		}
	}
	return space
}

func without(columns []column, dropped column) []column {
	kept := make([]column, 0, len(columns))
	for _, col := range columns {
		if col != dropped {
			kept = append(kept, col)
		}
	}
	return kept
}

// padded aligns text within a column: ages sit on the right, everything else
// on the left.
func padded(text string, width int, col column) string {
	if col == colCreated || col == colModified {
		return padLeft(text, width)
	}
	return padRight(text, width)
}

// styledCell renders one cell: the content is styled first and padded after,
// so that faint or struck-through styling never bleeds into the gap between
// columns.
func styledCell(doc docs.Document, col column, width int) string {
	text := truncate(plainCell(doc, col), width)
	faded := docs.Inactive(doc.Status)

	var styled string
	switch col {
	case colNumber:
		style := pathStyle
		if faded {
			style = style.Faint(true)
		}
		styled = hyperlink(doc.AbsPath, style.Render(text))
	case colPath:
		style := pathStyle
		if faded {
			style = faintStyle
		}
		styled = hyperlink(doc.AbsPath, style.Render(text))
	case colStatus:
		if doc.Status == "" {
			styled = text
			break
		}
		style := styleFor(doc.Status, statusStyles)
		if faded {
			style = style.Bold(false).Faint(true)
		}
		styled = style.Render(text)
	case colPriority:
		style := styleFor(doc.Priority, priorityStyles)
		if faded {
			style = faintStyle
		}
		styled = style.Render(text)
	case colType:
		style, known := typeStyles[doc.Type]
		if !known || faded {
			style = style.Faint(true)
		}
		styled = style.Render(text)
	case colTitle:
		switch {
		case doc.Err != nil:
			styled = errorStyle.Render(text)
		case doc.Status == "superseded":
			styled = titleStyle.Faint(true).Strikethrough(true).Render(text)
		case faded:
			styled = titleStyle.Faint(true).Render(text)
		default:
			styled = titleStyle.Render(text)
		}
	default:
		styled = faintStyle.Render(text)
	}
	return padded(styled, width, col)
}

// styleFor is the style for a known value, or the plain style for one the
// convention does not name.
func styleFor(value string, styles map[string]lipgloss.Style) lipgloss.Style {
	if style, ok := styles[value]; ok {
		return style
	}
	return lipgloss.NewStyle()
}

// countsOrder is the order statuses are summarised in, matching the order
// wj next considers them actionable, then the terminal statuses.
var countsOrder = []string{"in-progress", "blocked", "todo", "planned", "open", "shelved", "done", "rejected", "superseded"}

// countsLine summarises the listing by status: the known statuses in
// convention order, then any unknown ones alphabetically, then the documents
// with no status at all. Zero counts are omitted.
func countsLine(list []docs.Document) string {
	counts := map[string]int{}
	references := 0
	for _, doc := range list {
		if doc.Status == "" {
			references++
			continue
		}
		counts[doc.Status]++
	}

	unknown := []string{}
	for status := range counts {
		if !slices.Contains(countsOrder, status) {
			unknown = append(unknown, status)
		}
	}
	sort.Strings(unknown)

	parts := []string{}
	for _, status := range append(append([]string{}, countsOrder...), unknown...) {
		if counts[status] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[status], status))
		}
	}
	if references > 0 {
		parts = append(parts, fmt.Sprintf("%d reference", references))
	}
	return strings.Join(parts, " · ")
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

// truncate cuts text to width display cells, ending in an ellipsis when it had
// to cut anything.
func truncate(text string, width int) string {
	if lipgloss.Width(text) <= width {
		return text
	}
	runes := []rune(text)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}
