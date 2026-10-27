---
type: task
status: done
---

# wj: design

`wj` is a CLI for the `docs/` convention used across `~/local/*` projects. It becomes the single place the convention is defined, generated and checked. Humans and agents both use it; agents discover it by running it, not by reading prose in AGENTS.md.

## Decisions

- Project `wjdoc`, binary `wj`. Go. Install with `go install`.
- Zero config. If config is ever needed it is `docs/wj.jsonc`.
- One convention for every project. The one variation is the optional type folders (below, [003](003-Type-folders-ref-and-task.md)), inferred from the tree rather than configured.
- Wrapper, not framework: no section skeletons, no required headings. Structure is the project's and the agent's business.
- Agents write doc files directly and rely on `wj lint`. No write-through-tool requirement.
- Lint runs on commit, never on edit.
- Non-TTY output is plain and stable. TTY output is pretty.
- No TUI. `wj web` is the one interactive surface: a local viewer and editor, kept minimal (list, filter, read, edit, set status and priority). No creating docs, no live preview.

## Convention

Filenames `NNN-Sentence-case-title.md` in `docs/`.

Layout is flat by default: every document directly in `docs/`. A project opts into type folders by having `docs/ref/` or `docs/task/`; wj infers the layout from the tree, as it infers the padding width. In a foldered project reference lives in `docs/ref/` and every other type, unknown types included, in `docs/task/`. The about-doc stays at the top of `docs/` in both layouts. Numbers are one sequence across all of `docs/`. No other folder, and no folder below `ref/` or `task/`, holds documents.

- The integer is the document's identity and is never reused or changed. Zero padding is presentation only. `wj` infers the padding width from the directory and allocates at that width; new projects start at three digits. When a width is exhausted `wj new` refuses and points at `wj repad --width N`, which pads every filename and rewrites links in docs and code in one commit.
- Slug is the title with spaces replaced by hyphens, in sentence case: first letter uppercase, the rest as written so proper nouns keep their capitals (`002-Prisma-to-Drizzle.md`). No type prefix in the filename; that is what frontmatter is for.
- `000` (or `00`) is the about-doc, `type: reference`, and is where a project says anything project-specific.

Frontmatter, YAML between `---` fences, first thing in the file:

- `type`, required. Known: `reference` (evergreen), `review` (point-in-time assessment), `task` (unit of work adding something new), `bug` (unit of work fixing existing behaviour). Unknown values lint as a warning, not an error, so the vocabulary can grow.
- `status`, required for `task`, `bug` and `review`, absent for `reference`. Known: `todo` (immediate), `planned` (longer term), `in-progress`, `blocked`, `open` (reviews only), `shelved` (parked indefinitely, tasks and bugs only), `done`, `rejected` (decided against, tasks and bugs only), `superseded`. Unknown values warn.
- `priority`, optional, for `task`, `bug` and `review`. Known: `p0` (drop everything), `p1` (high), `p2` (normal), `p3` (low), `p4` (nice to have). Unknown values warn. See [002](002-Tags-and-priority.md).
- `tags`, optional, any type. A free-form list of lowercase `name` or `namespace:value`, e.g. `tags: [area:engine, reach:app]`.
- No `created`, `updated`, `author` fields: git already knows.
- Reviews state their assessed date and commit in the body. Not a frontmatter field yet; revisit if querying it becomes wanted.

Title is the first `# ` heading; falls back to the slug.

Links between docs are relative markdown links to the file, so a link across folders reads `../ref/NNN-slug.md`. Links from code and agent files use the path from the repository root: `docs/NNN-slug.md`, or `docs/ref/NNN-slug.md` in a foldered project.

## Commands

Discovery and reading:

- `wj list` (alias `ls`) `[-t/--type t] [-s/--status s,s] [-p/--priority p,p] [--tag t] [--all]` table of path, type, status, title. `--all` walks sibling repos (`../*/docs`) for a cross-project view.
- `wj show <ref> [--body]` (alias `cat`) prints a doc; `--body` strips frontmatter. `<ref>` is an integer, a padded number, a slug fragment or a path; ambiguity is an error listing the candidates.
- `wj next` lists actionable docs: `in-progress`, then `blocked` with their reason line, then `todo`, then `planned`, by priority within each. Answers "what should I pick up".
- `wj tags` lists the tags in use with their counts.
- `wj links <ref>` and `wj backlinks <ref>` from the link graph; backlinks include code and agent files.
- `wj search <term> [-t] [-s] [-E/--regex]` runs ripgrep over `docs/` with frontmatter filters applied first.
- `wj conventions` prints the convention above. `wj schema` prints it as JSON.

Writing:

- `wj new <type> <title>` allocates the next number and writes frontmatter and title, in a foldered project into the folder that suits the type. Max number is taken over every local and remote branch via `git ls-tree -r`, so parallel branches do not collide.
- `wj set <ref> <key> <value>` edits one frontmatter key in place without touching the body. `wj set <ref> status done` is the common case. In a foldered project, a `type` change that leaves the doc in the wrong folder prints the `wj move` that fixes it; `--move` does the move.
- `wj tag <ref> <tag>... [--remove]` adds or removes tags, writing `tags` as one flow-style line.
- `wj repad --width N [-n/--dry-run]` as above.
- `wj move <ref>... [--to ref|task|top] [--all] [-n/--dry-run]` moves documents to the folder that suits their type, or to `--to`, and rewrites every link to and from them: links between docs are recomputed from where each file will be, keeping anchors, titles and a leading `./`, and `docs/`-prefixed paths are rewritten across the repository. `--all` moves every doc not where it belongs, which migrates a flat project. A type folder it empties is removed.
- `wj init` creates `docs/` and the about-doc, appends one line to AGENTS.md or CLAUDE.md if present, and installs the pre-commit lint hook. Idempotent, safe on existing repos.

Checking:

- `wj lint [paths]` exit 1 on errors, 0 with warnings. Errors: missing or unparseable frontmatter, missing `type`, missing `status` where required, `tags` not a list of strings, `priority` not a single value, duplicate number, broken relative link, filename not matching the pattern (including a lowercase first letter after the number). Warnings: unknown type, status or priority, `status` or `priority` on a reference, a badly shaped or repeated tag, mixed padding widths, link to a superseded doc from an active one (inactive docs are history, so their links stay as written), markdown in a folder wj does not read. In a foldered project only: a doc in the folder that does not suit its type, a numbered doc at the top level other than the about-doc, the about-doc inside a folder.
- Output format on non-TTY is `path:line: level: message`, one per line, so agents and editors can parse it.

Git awareness (all read-only, shelling out to `git`):

- `list` gains a last-touched column from `git log -1`.
- `show` header line reports created and last-modified dates.
- `history <ref>` lists commits touching the doc with any frontmatter status change highlighted.

## Output

TTY: a coloured table, columns in this order.

- `#`, the padded number, bold and an OSC 8 `file://` link to the document. A file that does not follow the numbering shows its filename here instead.
- `Status`, coloured; empty for references rather than a dash.
- `Pri`, coloured, only when a listed document has a priority.
- `Title`, truncated with an ellipsis to whatever width is left, never narrower than 12 cells.
- `Type`, coloured as in `wj web` (reference cyan, review magenta, task blue, bug orange); unknown types faint.
- `Created` and `Modified`, relative ages, right aligned, faint.

`--paths` inserts a `Document` column of the repo-relative path after `#`. `--tags` inserts a `Tags` column after `Title`.

Narrow terminals drop columns before the title is truncated: `Type` first, then `Tags`, `Created`, `Modified` and `Pri`, each dropped only while `Title` would otherwise have under 30 cells. `#`, `Status`, `Title` and `Document` are never dropped.

Rows with status `shelved`, `done`, `rejected` or `superseded` are entirely faint, and a superseded title is struck through. A document with unreadable frontmatter shows its error in place of the title.

The table closes with one faint line of counts by status: in-progress, blocked, todo, planned, open, shelved, done, rejected, superseded, then any unknown statuses alphabetically, then `N reference`. Zero counts are omitted. Invalid-frontmatter error lines follow it.

`list` orders by number by default and takes `--sort number|created|modified|status|priority`, `--reverse`/`-r`, `--since <window>` (Go durations plus `d`, `w`, `mo`, `y` suffixes) and `--active`/`-a` (drop shelved, done, rejected and superseded). `next` keeps its own ordering and takes `--paths` only.

Created and modified come from one `git log` over `docs/`, following renames; a pure rename is not a modification.

Non-TTY or `--plain`: tab-separated, no colour, no decoration: `path`, `type`, `status`, `title`, `created`, `modified`, `priority`, `tags` (comma-separated), the dates ISO or empty.
`--json` on `list`, `show`, `next`, `lint`, `links`: stable field names `number, path, type, status, priority, tags, title, created, modified, error`.

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
