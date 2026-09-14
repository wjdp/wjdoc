// Package repad changes the zero-padding width of every numbered document and
// rewrites the references to their old filenames.
package repad

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wjdp/wjdoc/internal/docs"
	"github.com/wjdp/wjdoc/internal/gitx"
)

// Rename is one document moving to its newly padded filename. Both paths are
// relative to the repository root and use forward slashes.
type Rename struct {
	From string
	To   string
}

// Edit is one textual reference to an old filename, on a 1-based line.
type Edit struct {
	Path string
	Line int
	From string
	To   string
}

// Plan is everything a repad would change.
type Plan struct {
	Renames []Rename
	Edits   []Edit
}

// searchExtensions and searchNames are the files walked when ripgrep is absent.
var (
	searchExtensions = []string{".md", ".ts", ".js", ".go", ".py", ".json", ".yaml", ".yml", ".toml", ".txt"}
	searchNames      = []string{"AGENTS.md", "CLAUDE.md"}
)

// replacement is one old spelling of a filename and what it becomes.
type replacement struct {
	from string
	to   string
}

// New works out the renames and reference edits needed to pad every numbered
// document to width digits. It fails when the width cannot hold a number in
// use, or when a new filename is already taken.
func New(root string, documents []docs.Document, width int) (Plan, error) {
	var plan Plan
	var replacements []replacement

	for _, document := range documents {
		if document.Number < 0 || document.Width == 0 {
			continue
		}

		padded, err := docs.Pad(document.Number, width)
		if err != nil {
			return Plan{}, fmt.Errorf("%s: %w", document.Path, err)
		}
		if document.Width == width {
			continue
		}

		newName := padded + "-" + document.Slug + ".md"
		newPath := path.Join(path.Dir(document.Path), newName)

		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(newPath))); err == nil {
			return Plan{}, fmt.Errorf("%s already exists, cannot rename %s onto it", newPath, document.Path)
		}

		plan.Renames = append(plan.Renames, Rename{From: document.Path, To: newPath})
		replacements = append(replacements, spellings(document.Path, newPath)...)
	}

	if len(replacements) == 0 {
		return plan, nil
	}

	sort.SliceStable(replacements, func(i, j int) bool {
		return len(replacements[i].from) > len(replacements[j].from)
	})

	edits, err := findEdits(root, replacements)
	if err != nil {
		return Plan{}, err
	}
	plan.Edits = edits
	return plan, nil
}

// spellings lists every way a reference can name the old file, paired with the
// matching new spelling: the bare filename, the docs/-prefixed path, and the
// %20-escaped forms when the name contains spaces.
func spellings(oldPath, newPath string) []replacement {
	oldName, newName := path.Base(oldPath), path.Base(newPath)

	found := []replacement{
		{oldPath, newPath},
		{oldName, newName},
	}
	if strings.ContainsRune(oldName, ' ') {
		escape := func(text string) string { return strings.ReplaceAll(text, " ", "%20") }
		found = append(found,
			replacement{escape(oldPath), escape(newPath)},
			replacement{escape(oldName), escape(newName)},
		)
	}
	return found
}

// findEdits locates every line in the repository mentioning an old filename.
func findEdits(root string, replacements []replacement) ([]Edit, error) {
	candidates, err := searchWithRipgrep(root, replacements)
	if err != nil {
		candidates, err = searchByWalking(root)
		if err != nil {
			return nil, err
		}
	}

	var edits []Edit
	for _, candidate := range candidates {
		for _, applied := range appliedTo(candidate.text, replacements) {
			edits = append(edits, Edit{
				Path: candidate.path,
				Line: candidate.line,
				From: applied.from,
				To:   applied.to,
			})
		}
	}

	sort.SliceStable(edits, func(i, j int) bool {
		if edits[i].Path != edits[j].Path {
			return edits[i].Path < edits[j].Path
		}
		return edits[i].Line < edits[j].Line
	})
	return edits, nil
}

// candidate is one line that may contain a reference.
type candidate struct {
	path string
	line int
	text string
}

func searchWithRipgrep(root string, replacements []replacement) ([]candidate, error) {
	args := []string{"--line-number", "--no-heading", "--fixed-strings", "--color=never", "--glob", "!.git"}
	for _, item := range replacements {
		args = append(args, "-e", item.from)
	}
	args = append(args, ".")

	command := exec.Command("rg", args...)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		if exit, isExit := err.(*exec.ExitError); isExit && exit.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}

	var found []candidate
	for _, row := range strings.Split(string(output), "\n") {
		if row == "" {
			continue
		}
		filePath, rest, ok := strings.Cut(row, ":")
		if !ok {
			continue
		}
		lineText, text, ok := strings.Cut(rest, ":")
		if !ok {
			continue
		}
		line, err := strconv.Atoi(lineText)
		if err != nil {
			continue
		}
		found = append(found, candidate{
			path: filepath.ToSlash(strings.TrimPrefix(filePath, "./")),
			line: line,
			text: text,
		})
	}
	return found, nil
}

func searchByWalking(root string) ([]candidate, error) {
	var found []candidate

	err := filepath.WalkDir(root, func(absPath string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !searchable(entry.Name()) {
			return nil
		}

		relative, err := filepath.Rel(root, absPath)
		if err != nil {
			return err
		}

		file, err := os.Open(absPath)
		if err != nil {
			return err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for line := 1; scanner.Scan(); line++ {
			found = append(found, candidate{
				path: filepath.ToSlash(relative),
				line: line,
				text: scanner.Text(),
			})
		}
		return scanner.Err()
	})
	if err != nil {
		return nil, err
	}
	return found, nil
}

func searchable(name string) bool {
	for _, wanted := range searchNames {
		if name == wanted {
			return true
		}
	}
	if strings.HasPrefix(name, "README") {
		return true
	}
	extension := strings.ToLower(filepath.Ext(name))
	for _, wanted := range searchExtensions {
		if extension == wanted {
			return true
		}
	}
	return false
}

// appliedTo lists the distinct replacements that match somewhere in the line,
// longest spelling first so that "docs/07-x.md" is not also seen as "07-x.md".
func appliedTo(line string, replacements []replacement) []replacement {
	var applied []replacement
	seen := map[string]bool{}

	for offset := 0; offset < len(line); {
		matched := false
		for _, item := range replacements {
			if !strings.HasPrefix(line[offset:], item.from) || !wholeWord(line, offset, len(item.from)) {
				continue
			}
			if !seen[item.from] {
				seen[item.from] = true
				applied = append(applied, item)
			}
			offset += len(item.from)
			matched = true
			break
		}
		if !matched {
			offset++
		}
	}
	return applied
}

// wholeWord reports whether the match at offset is bounded by something other
// than a letter or a digit, so that a longer filename is never half-replaced.
func wholeWord(line string, offset, length int) bool {
	if offset > 0 {
		before, _ := utf8.DecodeLastRuneInString(line[:offset])
		if isWordRune(before) {
			return false
		}
	}
	if end := offset + length; end < len(line) {
		after, _ := utf8.DecodeRuneInString(line[end:])
		if isWordRune(after) {
			return false
		}
	}
	return true
}

func isWordRune(letter rune) bool {
	return unicode.IsLetter(letter) || unicode.IsDigit(letter)
}

// String is the dry-run summary: the renames, then the reference edits.
func (p Plan) String() string {
	var builder strings.Builder
	for _, rename := range p.Renames {
		fmt.Fprintf(&builder, "%s → %s\n", rename.From, rename.To)
	}
	for _, edit := range p.Edits {
		fmt.Fprintf(&builder, "%s:%d: %s → %s\n", edit.Path, edit.Line, edit.From, edit.To)
	}
	return builder.String()
}

// Empty reports whether the plan would change nothing.
func (p Plan) Empty() bool {
	return len(p.Renames) == 0 && len(p.Edits) == 0
}

// Apply rewrites the references first, so that no edit has to chase a file it
// has just moved, and then renames the documents.
func Apply(root string, plan Plan) error {
	if err := applyEdits(root, plan.Edits); err != nil {
		return err
	}
	return applyRenames(root, plan.Renames)
}

func applyEdits(root string, edits []Edit) error {
	byPath := map[string][]Edit{}
	var order []string
	for _, edit := range edits {
		if _, seen := byPath[edit.Path]; !seen {
			order = append(order, edit.Path)
		}
		byPath[edit.Path] = append(byPath[edit.Path], edit)
	}

	for _, filePath := range order {
		absPath := filepath.Join(root, filepath.FromSlash(filePath))
		source, err := os.ReadFile(absPath)
		if err != nil {
			return err
		}
		info, err := os.Stat(absPath)
		if err != nil {
			return err
		}

		lines := strings.Split(string(source), "\n")
		for _, edit := range byPath[filePath] {
			index := edit.Line - 1
			if index < 0 || index >= len(lines) {
				return fmt.Errorf("%s has no line %d", filePath, edit.Line)
			}
			lines[index] = replaceWholeWord(lines[index], edit.From, edit.To)
		}

		if err := os.WriteFile(absPath, []byte(strings.Join(lines, "\n")), info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

func replaceWholeWord(line, from, to string) string {
	var builder strings.Builder
	for offset := 0; offset < len(line); {
		if strings.HasPrefix(line[offset:], from) && wholeWord(line, offset, len(from)) {
			builder.WriteString(to)
			offset += len(from)
			continue
		}
		builder.WriteByte(line[offset])
		offset++
	}
	return builder.String()
}

func applyRenames(root string, renames []Rename) error {
	tracked := isGitRepository(root)

	for _, rename := range renames {
		if tracked {
			if _, err := gitx.Run(root, "mv", rename.From, rename.To); err == nil {
				continue
			}
		}
		from := filepath.Join(root, filepath.FromSlash(rename.From))
		to := filepath.Join(root, filepath.FromSlash(rename.To))
		if err := os.Rename(from, to); err != nil {
			return err
		}
	}
	return nil
}

func isGitRepository(root string) bool {
	output, err := gitx.Run(root, "rev-parse", "--is-inside-work-tree")
	return err == nil && output == "true"
}
