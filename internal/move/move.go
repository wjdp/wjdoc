// Package move moves documents between docs/ and its type folders and rewrites
// every link and reference to them.
package move

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/wjdp/wjdoc/internal/docs"
	"github.com/wjdp/wjdoc/internal/links"
	"github.com/wjdp/wjdoc/internal/repad"
)

var schemePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)

// Top names the top level of docs/ as a destination.
const Top = ""

// Plan is everything a move would change: the renames, the edited references,
// and the rewritten source of each file they touch.
type Plan struct {
	repad.Plan
	contents map[string]string
}

// Destination is the path document would have in folder, or at the top of
// docs/ when folder is Top.
func Destination(document docs.Document, folder string) string {
	return path.Join("docs", folder, path.Base(document.Path))
}

// Belongs is the folder document should sit in: the top for the about-doc,
// otherwise the folder that suits its type.
func Belongs(document docs.Document) string {
	if document.IsAboutDoc() {
		return Top
	}
	return docs.FolderFor(document.Type)
}

// New works out the edits needed to move each document from the key path to
// the value path, both repo-relative. Links between documents are rewritten
// from where each file will be; docs/-prefixed paths are rewritten everywhere
// else in the repository.
func New(root string, documents []docs.Document, moves map[string]string) (Plan, error) {
	plan := Plan{contents: map[string]string{}}
	if err := validate(root, moves); err != nil {
		return Plan{}, err
	}
	for _, document := range documents {
		if to, moving := moves[document.Path]; moving {
			plan.Renames = append(plan.Renames, repad.Rename{From: document.Path, To: to})
		}
	}
	if len(plan.Renames) == 0 {
		return plan, nil
	}

	newPath := func(old string) string {
		if to, moving := moves[old]; moving {
			return to
		}
		return old
	}

	for _, document := range documents {
		source, err := os.ReadFile(document.AbsPath)
		if err != nil {
			return Plan{}, err
		}
		rewritten, edits := rewriteLinks(document.Path, newPath(document.Path), string(source), newPath)
		if len(edits) > 0 {
			plan.contents[document.Path] = rewritten
			plan.Edits = append(plan.Edits, edits...)
		}
	}

	if err := plan.addReferences(root, moves); err != nil {
		return Plan{}, err
	}
	sort.SliceStable(plan.Edits, func(i, j int) bool {
		if plan.Edits[i].Path != plan.Edits[j].Path {
			return plan.Edits[i].Path < plan.Edits[j].Path
		}
		return plan.Edits[i].Line < plan.Edits[j].Line
	})
	return plan, nil
}

func validate(root string, moves map[string]string) error {
	taken := map[string]string{}
	for from, to := range moves {
		if other, clash := taken[to]; clash {
			return fmt.Errorf("%s and %s would both move to %s", other, from, to)
		}
		taken[to] = from
		if _, moving := moves[to]; moving {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(to))); err == nil {
			return fmt.Errorf("%s already exists, cannot move %s onto it", to, from)
		}
	}
	return nil
}

// addReferences rewrites docs/-prefixed spellings of the moved paths, in code,
// agent files and docs alike, on top of any link rewriting already planned.
func (p *Plan) addReferences(root string, moves map[string]string) error {
	var replacements []repad.Replacement
	for from, to := range moves {
		replacements = append(replacements, repad.Replacement{From: from, To: to})
		if strings.ContainsRune(from, ' ') {
			escape := func(text string) string { return strings.ReplaceAll(text, " ", "%20") }
			replacements = append(replacements, repad.Replacement{From: escape(from), To: escape(to)})
		}
	}

	found, err := repad.FindEdits(root, replacements)
	if err != nil {
		return err
	}
	for _, edit := range found {
		source, err := p.source(root, edit.Path)
		if err != nil {
			return err
		}
		lines := strings.Split(source, "\n")
		index := edit.Line - 1
		if index < 0 || index >= len(lines) {
			return fmt.Errorf("%s has no line %d", edit.Path, edit.Line)
		}
		replaced := repad.ReplaceWholeWord(lines[index], edit.From, edit.To)
		if replaced == lines[index] {
			continue
		}
		lines[index] = replaced
		p.contents[edit.Path] = strings.Join(lines, "\n")
		p.Edits = append(p.Edits, edit)
	}
	return nil
}

func (p *Plan) source(root, relative string) (string, error) {
	if source, planned := p.contents[relative]; planned {
		return source, nil
	}
	source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	return string(source), err
}

// Apply writes the rewritten files, then moves the documents.
func Apply(root string, plan Plan) error {
	for relative, source := range plan.contents {
		absPath := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Stat(absPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(absPath, []byte(source), info.Mode().Perm()); err != nil {
			return err
		}
	}
	if err := repad.ApplyRenames(root, plan.Renames); err != nil {
		return err
	}
	removeEmptiedFolders(root, plan.Renames)
	return nil
}

// removeEmptiedFolders deletes a type folder that a move left empty, since an
// empty docs/ref/ or docs/task/ would still make the project read as foldered.
func removeEmptiedFolders(root string, renames []repad.Rename) {
	for _, rename := range renames {
		if folder := path.Dir(rename.From); folder != "docs" {
			_ = os.Remove(filepath.Join(root, filepath.FromSlash(folder)))
		}
	}
}

// rewriteLinks recomputes every relative link in a document that is moving
// from oldPath to newPath, or that links to a document that is moving, keeping
// each link's anchor, title and form.
func rewriteLinks(oldPath, newPath, source string, newPathOf func(string) string) (string, []repad.Edit) {
	lines := strings.Split(source, "\n")
	var edits []repad.Edit

	destinations := links.Destinations(source)
	for index := len(destinations) - 1; index >= 0; index-- {
		destination := destinations[index]
		start, end, ok := linkPath(destination.Raw)
		if !ok {
			continue
		}
		written := destination.Raw[start:end]
		target := links.ResolveTarget(oldPath, strings.ReplaceAll(written, "%20", " "))
		if newPath == oldPath && newPathOf(target) == target {
			continue
		}
		rewritten := relativeLink(path.Dir(newPath), newPathOf(target), written)
		if rewritten == written {
			continue
		}

		line := lines[destination.Line-1]
		at := destination.Start + start
		lines[destination.Line-1] = line[:at] + rewritten + line[destination.Start+end:]
		edits = append(edits, repad.Edit{Path: oldPath, Line: destination.Line, From: written, To: rewritten})
	}

	for left, right := 0, len(edits)-1; left < right; left, right = left+1, right-1 {
		edits[left], edits[right] = edits[right], edits[left]
	}
	return strings.Join(lines, "\n"), edits
}

// linkPath finds the file path within a raw link destination, leaving out any
// angle brackets, title and "#fragment". It reports false for destinations that
// name no relative file: URLs, absolute paths and bare fragments.
func linkPath(raw string) (start, end int, ok bool) {
	start = len(raw) - len(strings.TrimLeft(raw, " \t"))
	end = len(strings.TrimRight(raw, " \t"))
	if strings.HasPrefix(raw[start:], "<") {
		start++
		if closing := strings.IndexByte(raw[start:], '>'); closing >= 0 {
			end = start + closing
		}
	} else if space := strings.IndexAny(raw[start:end], " \t"); space >= 0 {
		end = start + space
	}
	if fragment := strings.IndexByte(raw[start:end], '#'); fragment >= 0 {
		end = start + fragment
	}

	written := raw[start:end]
	if written == "" || strings.HasPrefix(written, "/") || schemePattern.MatchString(written) {
		return 0, 0, false
	}
	return start, end, true
}

// relativeLink is the link from a file in directory to target, written in the
// same form as before: a leading "./", %20 for spaces and a trailing "/" are kept.
func relativeLink(directory, target, before string) string {
	relative, err := filepath.Rel(filepath.FromSlash(directory), filepath.FromSlash(target))
	if err != nil {
		return before
	}
	relative = filepath.ToSlash(relative)

	if strings.HasPrefix(before, "./") && !strings.HasPrefix(relative, "../") {
		relative = "./" + relative
	}
	if strings.HasSuffix(before, "/") && !strings.HasSuffix(relative, "/") {
		relative += "/"
	}
	if strings.Contains(before, "%20") {
		relative = strings.ReplaceAll(relative, " ", "%20")
	}
	if path.Clean(relative) == path.Clean(before) {
		return before
	}
	return relative
}
