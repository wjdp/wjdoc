---
type: task
status: done
---

# Tags and priority

Two optional frontmatter fields, so docs can be sliced by area and ranked within a status.

```yaml
---
type: task
status: todo
priority: p1
tags: [area:engine, reach:app, impact:figures]
---
```

## Convention

`priority`, optional, for task, bug and review. Known: `p0` (drop everything), `p1` (high), `p2` (normal), `p3` (low), `p4` (nice to have). Absent means unprioritised and ranks after `p4`. Unknown values warn, as with status. On a reference it warns, like `status` does.

`tags`, optional, any type. A YAML list of strings, flow or block style, quoted or not; `wj` writes flow style, unquoted where YAML allows (`[area:engine, reach:app]`; goccy parses that bare). Shape: lowercase `name` or `namespace:value`, characters `[a-z0-9-]`. Free-form: no fixed namespaces and no per-project vocabulary to enforce; `wj tags` is how the vocabulary in use is discovered.

Priority is its own field rather than a `priority:p1` tag because it orders `next` and `list --sort`, and a scalar is easier to set, filter and compare than a list member.

## Lint

Errors: `tags` that is not a list of strings (a bare scalar `tags: foo` included), `priority` that is not a scalar.

Warnings: unknown priority, priority on a reference, a tag not matching the shape, a duplicate tag.

## Commands

- `list` gains `-p/--priority p0,p1` and `--tag t` (repeatable). Several `--tag`s must all match. A tag ending in `:` matches the namespace, so `--tag area:` keeps everything with any `area:` tag. `--sort priority` ranks p0 to p4 then unprioritised, number breaking ties.
- `next` keeps its status order and ranks by priority within each status, then number. It only reorders: no priority is hidden. Takes `--tag` and `--priority` so `wj next --tag area:engine` works.
- `search` takes `--tag` and `--priority` like `list`.
- `set` handles priority unchanged: `wj set 7 priority p1`.
- New `wj tag <ref> <tag>...` adds tags, `--remove` removes them. Input is lowercased; a tag not matching the shape is refused (lint only warns, for hand edits). Rewrites the `tags` key as one flow-style line, consuming a block list's continuation lines, leaving the rest of the file byte for byte. Removing the last tag drops the key. Keeps first-seen order, no sorting.
- New `wj tags` lists every tag in use with its usage count, grouped by namespace, most used first within each. Takes `--all` and `--active`. Lets an agent find the existing vocabulary before inventing a tag.
- `show` header line adds priority and tags.

## Output

TTY table: a `Pri` column after `Status`, shown by default whenever a listed doc has a priority and omitted otherwise. `p0` bold red, `p1` yellow, `p2` plain, `p3`/`p4` faint. Tags are not a table column by default (too wide); `--tags` on `list` and `next` adds a `Tags` column after `Title`, comma-separated and truncated with an ellipsis. Narrow terminals drop `Type`, then `Tags`, `Created`, `Modified`, and `Pri` last.

Plain: append `priority` and `tags` columns after `modified`, tags comma-separated. Appending keeps existing column positions stable for scripts.

JSON: `priority` (string, empty when absent) and `tags` (array, empty not null) on `list`, `show`, `next`.

Schema: `wj schema` lists `priority` and `tags` under optional frontmatter, the known priorities, and the tag pattern. `wj conventions` gains matching prose.

## Web

`wj web` shows priority and tags on list and read views, filters by both (priority and tag selects, kept in the query string like the others; the search box also matches tags), and sets priority alongside status. Adding and removing tags in the web editor is out of scope for now; edit the raw frontmatter.

## Implementation

- `docs.Document` gains `Priority string` and `Tags []string`, parsed in `load.go`; a malformed `tags` keeps the document and records the problem for lint.
- `docs.KnownPriorities` next to `KnownStatuses`.
- `docs.SetList(source, key, values)` beside `SetField`: finds the key line, extends over indented `- ` continuation lines, replaces them with one flow line.
- `docs.Filter` takes a `docs.Criteria` of types, statuses, priorities and tags.
- `SetField` and `UnsetField` also consume a key's continuation lines, so setting or unsetting a key that held a block list no longer leaves orphaned `- ` lines.

## Plan

1. Parse, JSON, plain output, lint, schema and conventions.
2. `list`/`next`/`search` filters and sorting, table column.
3. `wj tag`, `wj tags`.
4. `show` header, web.
5. Update 001-Design.md, README and skill.

## Resolved questions

- Five levels, `p0`–`p4`.
- `next` only reorders by priority, never hides.
- Tags are free-form; no enforced vocabulary.
- `wj tags` lists tags with usage counts.
