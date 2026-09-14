---
type: task
status: done
---

# wj: design

`wj` is a CLI for the `docs/` convention used across `~/local/*` projects. It becomes the single place the convention is defined, generated and checked. Humans and agents both use it; agents discover it by running it, not by reading prose in AGENTS.md.

## Decisions

- Project `wjdoc`, binary `wj`. Go. Install with `go install`.
- Zero config. If config is ever needed it is `docs/wj.jsonc`.
- One convention for every project, no per-project variation.
- Wrapper, not framework: no section skeletons, no required headings. Structure is the project's and the agent's business.
- Agents write doc files directly and rely on `wj lint`. No write-through-tool requirement.
- Lint runs on commit, never on edit.
- Non-TTY output is plain and stable. TTY output is pretty.
- No TUI.

## Convention

Filenames `NNN-Sentence-case-title.md` in `docs/`.

- The integer is the document's identity and is never reused or changed. Zero padding is presentation only. `wj` infers the padding width from the directory and allocates at that width; new projects start at three digits. When a width is exhausted `wj new` refuses and points at `wj repad --width N`, which pads every filename and rewrites links in docs and code in one commit.
- Slug is the title with spaces replaced by hyphens, in sentence case: first letter uppercase, the rest as written so proper nouns keep their capitals (`002-Prisma-to-Drizzle.md`). No type prefix in the filename; that is what frontmatter is for.
- `000` (or `00`) is the about-doc, `type: reference`, and is where a project says anything project-specific.

Frontmatter, YAML between `---` fences, first thing in the file:

- `type`, required. Known: `reference` (evergreen), `review` (point-in-time assessment), `task` (unit of work). Unknown values lint as a warning, not an error, so the vocabulary can grow.
- `status`, required for `task` and `review`, absent for `reference`. Known: `todo` (immediate), `planned` (longer term), `in-progress`, `blocked`, `open` (reviews only), `done`, `superseded`. Unknown values warn.
- No `created`, `updated`, `author` fields: git already knows.
- Reviews state their assessed date and commit in the body. Not a frontmatter field yet; revisit if querying it becomes wanted.

Title is the first `# ` heading; falls back to the slug.

Links between docs are relative markdown links to the filename. Links from code and agent files use `docs/NNN-slug.md`.

## Commands

Discovery and reading:

- `wj list [--type t] [--status s,s] [--all]` table of path, type, status, title. `--all` walks sibling repos (`../*/docs`) for a cross-project view.
- `wj show <ref> [--body]` prints a doc; `--body` strips frontmatter. `<ref>` is an integer, a padded number, a slug fragment or a path; ambiguity is an error listing the candidates.
- `wj next` lists actionable docs: `in-progress`, then `blocked` with their reason line, then `todo`, then `planned`. Answers "what should I pick up".
- `wj links <ref>` and `wj backlinks <ref>` from the link graph; backlinks include code and agent files.
- `wj search <term>` runs ripgrep over `docs/` with frontmatter filters applied first.
- `wj conventions` prints the convention above. `wj schema` prints it as JSON.

Writing:

- `wj new <type> <title>` allocates the next number and writes frontmatter and title. Max number is taken over every local and remote branch via `git ls-tree`, so parallel branches do not collide.
- `wj set <ref> <key> <value>` edits one frontmatter key in place without touching the body. `wj set <ref> status done` is the common case.
- `wj repad --width N` as above.
- `wj init` creates `docs/` and the about-doc, appends one line to AGENTS.md or CLAUDE.md if present, and installs the pre-commit lint hook. Idempotent, safe on existing repos.

Checking:

- `wj lint [paths]` exit 1 on errors, 0 with warnings. Errors: missing or unparseable frontmatter, missing `type`, missing `status` where required, duplicate number, broken relative link, filename not matching the pattern (including a lowercase first letter after the number). Warnings: unknown type or status, `status` on a reference, mixed padding widths, link to a superseded doc.
- Output format on non-TTY is `path:line: level: message`, one per line, so agents and editors can parse it.

Git awareness (all read-only, shelling out to `git`):

- `list` gains a last-touched column from `git log -1`.
- `show` header line reports created and last-modified dates.
- `history <ref>` lists commits touching the doc with any frontmatter status change highlighted.

## Output

TTY: colour table, path as a clickable `file://` link, type and status coloured, title column sized to the remaining width and truncated, dropped only when under 12 columns remain. Same palette as the existing script.
Non-TTY or `--plain`: tab-separated, no colour, no decoration.
`--json` on `list`, `show`, `next`, `lint`, `links`: stable field names `number, path, type, status, title, modified`.

## Agent integration

- The one line `init` writes: `Docs in docs/, managed by wj. Run wj conventions before reading or writing them.`
- Optional global Claude Code skill shipped in this repo under `skill/`, installed by `wj setup` as a symlink into `~/.claude/skills/`. Describes the workflow (`next` at session start, `new` for write-ups, `set status` on finish). Not required for the tool to be useful.
- Pre-commit hook from `init` runs `wj lint` on staged docs only.

## Implementation

- Go, `cobra` for commands, `goccy/go-yaml` for frontmatter, `goldmark` for link extraction, `lipgloss` for tables. Standard `internal/` layout: `docs` (discover, parse, refs), `lint`, `render`, `gitx`.
- Frontmatter edits (`set`) rewrite only the fence block and preserve the rest of the file byte for byte.
- Tests are table-driven against fixture directories copied from the three real repos.

## Plan

All delivered 2026-09-14:

1. Core: discover, parse, `list`, `show`, plain and pretty output.
2. `lint` and `new`.
3. `set`, `next`, `links`, `backlinks`, `init` with hook.
4. `--all`, git columns, `history`, `repad`.
5. Skill and `setup`.

## Resolved questions

- `--all` walks the parent directory of the current repo, or `$WJ_PROJECTS` when set, and skips folders whose `docs/` has no numbered files.
- `lint` checks docs-to-docs links only. Code links are covered by `backlinks` and rewritten by `repad`.
- `wj new` never allocates below the current maximum, so a second about-doc cannot happen by accident.
