package docs

import (
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// The folders of a foldered project: evergreen reference in one, every other
// type in the other. A project without either is flat.
const (
	RefFolder  = "ref"
	TaskFolder = "task"
)

// Folders lists the recognised subfolders of docs/, in loading order.
var Folders = []string{RefFolder, TaskFolder}

// Foldered reports whether the project at root has opted into type folders by
// having docs/ref/ or docs/task/.
func Foldered(root string) bool {
	for _, folder := range Folders {
		if isDir(filepath.Join(root, "docs", folder)) {
			return true
		}
	}
	return false
}

// FolderFor is the folder a document of this type belongs in.
func FolderFor(docType string) string {
	if strings.EqualFold(strings.TrimSpace(docType), "reference") {
		return RefFolder
	}
	return TaskFolder
}

// Folder is the recognised folder a document sits in, or "" at the top of docs/.
func (d Document) Folder() string {
	directory := path.Dir(d.Path)
	if directory == "docs" {
		return ""
	}
	return path.Base(directory)
}

// IsAboutDoc reports whether the document is the about-doc, number 0, which
// stays at the top of docs/ in every layout.
func (d Document) IsAboutDoc() bool {
	return d.Number == 0
}

// Unloaded lists the repo-relative markdown files below docs/ that Load does
// not read: those in any folder other than ref/ and task/, and in any folder
// nested below those.
func Unloaded(root string) ([]string, error) {
	docsDir := filepath.Join(root, "docs")
	var found []string
	err := filepath.WalkDir(docsDir, func(absPath string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !isMarkdown(entry.Name()) {
			return nil
		}
		relative, err := filepath.Rel(docsDir, absPath)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(relative), "/")
		if len(parts) == 1 || len(parts) == 2 && isRecognisedFolder(parts[0]) {
			return nil
		}
		found = append(found, "docs/"+filepath.ToSlash(relative))
		return nil
	})
	return found, err
}

func isRecognisedFolder(name string) bool {
	return slices.Contains(Folders, name)
}

func isMarkdown(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".md")
}
