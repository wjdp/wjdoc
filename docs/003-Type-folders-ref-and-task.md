---
type: task
status: in-progress
---

# Type folders ref and task

An optional layout that splits `docs/` into two folders by type: `docs/ref/` for evergreen reference and `docs/task/` for every other type (task, bug, review and unknown types). Asked for by ttsd (`~/local/ttsd/docs/196-Keep-docs-manageable-as-the-folder-grows.md`, "Decided"), at about 200 docs. There, 18 living reference docs are mixed in with the log of work, and one of them (012, 12.6k words, 77 edits) is the most-linked doc in the repo. Filtering by type helps, but it does not separate them in the file tree, in editors or in `rg`.

Reviews go under `task/` because a review closes, as a task does. Tasks and bugs are not split, because splitting them would gain nothing.

## Backwards compatible

A flat project is unchanged and needs nothing. Every command, output shape and lint result on a flat `docs/` is the same as today. The three existing flat projects (tetanus, grate, wjdp-os) must pass the full test suite and `wj lint` byte for byte before and after.

A project opts in by having `docs/ref/` or `docs/task/`, as there is no config (001, "Zero config"). wj infers the layout from the tree, in the way it infers the padding width. An empty folder is not tracked by git but still counts, so the layout can differ between a working tree and a fresh clone until a doc lands in it.

- **Flat:** neither folder exists. Docs live directly in `docs/`, as now.
- **Foldered:** either folder exists. `wj new` writes into the folder that suits the type.
- **Mixed:** a foldered project that still has numbered docs at the top level. Every command keeps working on them, so a project can migrate in steps or on a branch. `wj new` writes into the folders, not the top level. Lint warns, once per doc, pointing at the migration command.

This is a second layout, which 001's decision "One convention for every project, no per-project variation" rules out. Amend that decision to allow the folders as the one optional variation, since forcing them on a project of a handful of docs is ceremony.

## Convention

- Numbers form one sequence across the whole of `docs/`. The number is still the identity, so `wj show 12` finds 12 in whichever folder it sits. `Resolve` already matches on number before path, and `pathBase` still matches a bare filename, so this comes from loading every folder.
- Reference keeps its number: `docs/ref/012-Deco-engine-conventions.md`.
- Padding width is inferred across both folders.
- Only `ref/` and `task/` are recognised. Nested folders below them are not recognised either. Keep it two levels until a project needs more.
- Links between docs stay relative, so a link across folders reads `../ref/012-….md`. Links from code and agent files use the full path, `docs/ref/012-….md`.
- The about-doc (`000`) stays at the top of `docs/` in every layout, as the entry point that `wj init` creates and the agent line names. It is the one reference outside `ref/`.

## Changes

Loading and identity:

- `docs.Load` reads `docs/`, `docs/ref/` and `docs/task/`, sorted by number then path, not by filename alone. `Document.Path` stays repo-relative and so gains the folder (`docs/ref/012-….md`). JSON and plain output keep their field names; only the path value differs, and only in foldered projects.
- `gitx.DocNamesAcrossBranches` lists `docs/` with `ls-tree -r`. Without it, `ls-tree --name-only ref docs/` returns the folder names, not the docs inside, and `wj new` on a foldered branch would reuse a number allocated on another branch. Base names stay the key, since numbers are global.
- `gitx.DocumentDates` already reads all of `docs/` with `-M`, so a move between folders is a pure rename (R100): it keeps the doc's created date and is not a modification. Exact renames are cheap for git to detect, so a 200-doc migration commit stays within the rename limit. Check this with a fixture.
- `gitx.History` and `StatusAt` lose past statuses after any rename, a bug `repad` already has. `history.go` calls `StatusAt(hash, currentPath)`, but commits before the rename hold the file at its old path, so `git show` fails and the status reads as empty. `History` must return each commit's path (`log --follow --name-only`), and `StatusAt` must read that path. `wj move --all` would hit this on every doc.
- `docs.Root` is unchanged: a project is still found by `docs/`.

Writing:

- `wj new <type>` in a foldered or mixed project writes `reference` to `docs/ref/` and every other type to `docs/task/`. In a flat project it writes to `docs/` as now. `--json` output's `path` reflects the folder.
- `wj set <ref> type …` does not move the file by default; lint flags the mismatch and `set` prints a hint naming `wj move`. With `--move`, `set` also moves the file to the folder that suits the new type, through the same code as `wj move`. `--move` is a no-op in a flat project and for `000`.
- `wj repad` mostly works already. `repad.New` keeps each file in its folder (`path.Dir`), the `docs/…` spelling carries the folder, and the bare-filename spelling rewrites `../ref/012-x.md` because `/` counts as a word boundary. Cover it with a foldered fixture rather than new code.
- `wj init` is unchanged: it creates a flat project. Opting in is creating a folder, or running the move command.
- The pre-commit hook's pathspec `'docs/*.md'` already matches subfolders, as `*` crosses `/` in a git pathspec. Add a test so it stays that way.

New command, to migrate and to fix a doc in the wrong folder:

- `wj move <ref>... [--to ref|task] [-n/--dry-run]` moves docs into their folders and rewrites every link affected. Uses `git mv` in a repository. With no `--to`, each doc goes to the folder that suits its type.
- `wj move --all` moves every top-level doc but `000`. It is the migration for a whole project, in one commit.
- Rewriting links is not repad's text replacement. `repad.spellings` swaps one string for another, which holds only while the directory stays the same. After a move, a relative link depends on where both the linking file and its target sit:
  - Links into a moved doc, from other docs: `012-x.md` from `task/` becomes `../ref/012-x.md`.
  - Links out of a moved doc: every relative link is recomputed from its new folder, to docs and to anything else (`../AGENTS.md` becomes `../../AGENTS.md`).
  - Both sides use `links.Extract` and `links.ResolveTarget`, then write the new relative path, keeping any `#anchor` and the link's own form (`./`, `%20`), so moving back restores the original bytes.
  - Code and agent files: `docs/NNN-…` to `docs/ref/NNN-…`, which is text replacement and can share repad's search (`findEdits` and its ripgrep path). Bare filenames in code are left alone, as they stay valid names.
- Share `applyEdits` and `applyRenames` with `repad`, and the plan's dry-run printing. This is new link-aware code, the bulk of the work, not a refactor of repad.

Lint, warnings only, and none fires on a flat project:

- A doc in the folder that does not suit its type (`reference` in `task/`, anything else in `ref/`).
- A numbered doc at the top level of a foldered project, other than `000`.
- An unrecognised subfolder of `docs/` holding markdown. This is new: `Load` skips directories today, so such files are ignored silently.
- The duplicate-number error already compares numbers across all loaded docs (`lint.duplicates`). Confirm it catches 012 in both `ref/` and `task/`.
- Broken links are already errors. After a hand move, lint reports every stale link, which is what points to `wj move`.

Reading:

- `list`, `next`, `search`, `links`, `backlinks`, `history`, `tags` and `show` work across both folders. `search` passes the loaded docs' paths to `rg`, and `list --all` loads each sibling with `docs.Load`, so both follow from loading.
- `--paths` shows the folder. The `#` column is unchanged.
- `backlinks` excludes doc files from the code side by the `docs/` prefix (`project/backlinks.go`), which still holds.
- `wj web`: the server addresses docs by filename (`byName`), which is unique where numbers are, and serves `docs/` recursively (`http.FileServer`). The front end does not cope: `docsURL` in `app.js` resolves every link and image against `/docs/`, so `../ref/012-….md` from a doc in `task/` lands on `/ref/…`, and `onViewClick` strips a fixed `/docs/` prefix to find the doc. Resolve against `/docs/` plus the current doc's folder, and match the linked doc by path.

Schema, conventions, skill:

- `wj conventions` and `wj schema` describe both layouts: flat by default, foldered when `ref/` or `task/` exists, which type goes where, `wj move` and `set --move`.
- `skill/SKILL.md` (and the embedded copy `wj setup` syncs) mentions that a doc's path may include its folder, and that moving a doc is `wj move` or `set --move`, never a hand `mv`.
- Update 001-Design.md (the "One convention" decision, "Convention", "Commands", lint lists) and the README.

## Tests

- Fixtures: keep the flat fixtures from the real repos. Add a foldered fixture and a mixed one, with links across folders, a doc linking out to a non-doc file, a link with an anchor, a code file linking to `docs/ref/…` and to an old flat path, and a doc in the wrong folder.
- Every command run against the flat fixtures gives output identical to master. Golden comparisons where tests already use them.
- `wj new` on a foldered fixture allocates past a number that exists only on another branch, inside a folder (the `ls-tree -r` case).
- `wj history` on a doc renamed by `repad` and by `move` shows the statuses from before the rename.
- `wj move --all` on a copy of the flat fixture, then `wj lint` clean and every link resolving. Moving back restores the original bytes, for a round-trip check.
- `wj set <ref> type reference --move` moves the doc and rewrites its links.
- `wj repad` on a foldered fixture.
- `wj web` follows a link across folders and shows an image from a doc in a folder.
- The pre-commit hook lints a staged doc in `docs/ref/`.
- Run on ttsd's `docs/` in a scratch worktree: `wj move --all --dry-run`, then the real move, `wj lint`, and `rg 'docs/[0-9]{3}-'` outside `docs/` finding nothing left to rewrite. ttsd has about 615 links between docs and 141 references from code.

## Plan

1. Load across folders, `ls-tree -r`, path-aware sorting, and `history` following renames. Flat output unchanged, checked by the existing tests.
2. `wj new` picks the folder. Lint warnings for the layout.
3. `wj move` with link-aware rewriting, sharing repad's apply and rename steps. Then `set --move` on top of it.
4. `repad` and `search` tests across folders; the `web` front-end fix.
5. Conventions, schema, skill, 001-Design.md, README.
6. Migrate ttsd in one commit, done by the ttsd owner. Not part of this task's branch.

## Out of scope

The other wj items from ttsd's doc 196 (`summary`, `caused-by` and `parent` fields, open docs by default, `group:` and `release:` tags, lint for drift, `wj index`) are separate tasks. Folders come first, because a migration that rewrites links is easier while fewer tools depend on paths.

## Resolved questions

- The layout is optional, inferred from the tree; 001's "One convention" decision is amended to allow it.
- `000` stays at the top of `docs/` in every layout.
- The folders are `ref` and `task`, fixed in the convention for every project.
- `wj set <ref> type …` moves the file only under `--move`; without it, it prints a hint naming `wj move`.
- In a mixed project, `wj new` writes into the folders.
