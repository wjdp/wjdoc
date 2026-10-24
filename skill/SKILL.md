---
name: wj-docs
description: Working in a repo with a docs/ folder managed by wj, or asked about task, design or review docs. Use for reading, writing, or updating docs/*.md.
---

# wj docs/

`docs/` holds numbered markdown documents (`NNN-Sentence-case-title.md`) with
YAML frontmatter: `type` (reference, review, task, bug) and, for all but
reference, `status`. `wj` is the CLI that reads, writes and checks them.

## Workflow

- Session start: `wj next` — what's actionable: in-progress, then blocked
  (with its reason), then todo, then planned.
- Read a doc: `wj show <ref> --body` (`<ref>` is a number, slug fragment or
  path).
- Write one up: `wj new task <title>` for something new, `wj new bug <title>`
  for a fix to existing behaviour, then edit the body of the file it creates
  directly.
- Prioritise and tag: `wj set <ref> priority p1` (p0–p4), `wj tag <ref>
  area:engine`. Run `wj tags` first and reuse an existing tag where one fits.
- Pick something up: `wj set <ref> status in-progress`.
- Finish it: `wj set <ref> status done`.
- Before committing: `wj lint`.
- A doc references code, or code references a doc: `wj links <ref>` /
  `wj backlinks <ref>`.
- Unsure of a rule: `wj conventions` (prose) or `wj schema` (JSON).

## Rules

- Never hand-write frontmatter for a new doc — `wj new` allocates the number
  and writes it. Editing an existing doc's frontmatter by hand is fine;
  `wj set` is the shortcut, and `wj lint` catches mistakes either way.
- Never renumber files. The number is the document's identity.
- Unknown `type` or `status` values are allowed; `wj lint` warns, it doesn't
  error.

Every command is plain text when piped and takes `--json` for structured
output.
