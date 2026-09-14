package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wjdp/wjdoc/internal/docs"
)

// AgentLine is the single line init adds to AGENTS.md or CLAUDE.md.
const AgentLine = "Docs in docs/, managed by wj. Run wj conventions before reading or writing them."

// hookMarker identifies a pre-commit hook wj installed, so that a hook written
// by anyone else is left alone.
const hookMarker = "# installed by wj"

// LintHook is the pre-commit hook: wj lint over the staged documents only.
const LintHook = `#!/bin/sh
` + hookMarker + `
files=$(git diff --cached --name-only --diff-filter=ACMR -- 'docs/*.md')
[ -z "$files" ] || exec wj lint $files
`

// Action is one thing init did, or deliberately did not do.
type Action struct {
	Path string
	Done string
	Note string
}

// Report is everything init did, in the order it happened.
type Report []Action

// Init creates docs/ and the about-doc, points the project's agent file at wj
// and installs the pre-commit lint hook. Every step is idempotent: a second run
// changes nothing and reports what was already there.
func Init(root string, projectName string) (Report, error) {
	if projectName == "" {
		projectName = filepath.Base(root)
	}

	var report Report
	docsAction, err := ensureDocsDir(root)
	if err != nil {
		return nil, err
	}
	report = append(report, docsAction)

	aboutAction, err := ensureAboutDoc(root, projectName)
	if err != nil {
		return nil, err
	}
	report = append(report, aboutAction)

	agentAction, err := ensureAgentLine(root)
	if err != nil {
		return nil, err
	}
	report = append(report, agentAction)

	hookAction, err := ensureHook(root)
	if err != nil {
		return nil, err
	}
	return append(report, hookAction), nil
}

func ensureDocsDir(root string) (Action, error) {
	action := Action{Path: "docs/", Done: "exists"}
	if isDir(filepath.Join(root, "docs")) {
		return action, nil
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		return Action{}, err
	}
	action.Done = "created"
	return action, nil
}

func ensureAboutDoc(root, projectName string) (Action, error) {
	documents, err := docs.Load(root)
	if err != nil {
		return Action{}, err
	}

	for _, document := range documents {
		if document.Number == 0 {
			return Action{Path: document.Path, Done: "exists"}, nil
		}
	}

	padded, err := docs.Pad(0, docs.Width(documents))
	if err != nil {
		return Action{}, err
	}

	path := filepath.ToSlash(filepath.Join("docs", padded+"-Docs.md"))
	source := aboutDocSource(projectName)
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte(source), 0o644); err != nil {
		return Action{}, err
	}
	return Action{Path: path, Done: "created"}, nil
}

func aboutDocSource(projectName string) string {
	return fmt.Sprintf(`---
type: reference
---

# About these docs

Working documentation for %s: plans, reviews and task write-ups, numbered in
creation order and managed by `+"`wj`"+`.

Run `+"`wj conventions`"+` for the rules.
`, projectName)
}

func ensureAgentLine(root string) (Action, error) {
	const agentsFile, claudeFile = "AGENTS.md", "CLAUDE.md"

	for _, name := range []string{agentsFile, claudeFile} {
		content, err := readIfPresent(filepath.Join(root, name))
		if err != nil {
			return Action{}, err
		}
		if strings.Contains(content, AgentLine) {
			return Action{Path: name, Done: "exists"}, nil
		}
	}

	target := agentsFile
	if !exists(filepath.Join(root, agentsFile)) && exists(filepath.Join(root, claudeFile)) {
		target = claudeFile
	}

	existing, err := readIfPresent(filepath.Join(root, target))
	if err != nil {
		return Action{}, err
	}

	done := "appended"
	if existing == "" {
		done = "created"
	}
	if err := os.WriteFile(filepath.Join(root, target), []byte(withAgentLine(existing)), 0o644); err != nil {
		return Action{}, err
	}
	return Action{Path: target, Done: done}, nil
}

func withAgentLine(existing string) string {
	if existing == "" {
		return AgentLine + "\n"
	}
	if !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}
	if !strings.HasSuffix(existing, "\n\n") {
		existing += "\n"
	}
	return existing + AgentLine + "\n"
}

func ensureHook(root string) (Action, error) {
	action := Action{Path: ".git/hooks/pre-commit"}
	if !isDir(filepath.Join(root, ".git")) {
		action.Done = "skipped"
		action.Note = "no git repository"
		return action, nil
	}

	hooksDir := filepath.Join(root, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return Action{}, err
	}

	hookPath := filepath.Join(hooksDir, "pre-commit")
	existing, err := readIfPresent(hookPath)
	if err != nil {
		return Action{}, err
	}
	switch {
	case strings.Contains(existing, hookMarker):
		action.Done = "exists"
		return action, nil
	case existing != "":
		action.Done = "skipped"
		action.Note = "foreign hook"
		return action, nil
	}

	if err := os.WriteFile(hookPath, []byte(LintHook), 0o755); err != nil {
		return Action{}, err
	}
	action.Done = "created"
	return action, nil
}

// ForeignHook reports whether init left an existing pre-commit hook alone, so
// that the caller can print the lines to add by hand.
func (r Report) ForeignHook() bool {
	for _, action := range r {
		if action.Note == "foreign hook" {
			return true
		}
	}
	return false
}

func readIfPresent(path string) (string, error) {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
