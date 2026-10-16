# wjdoc

Dumb as bricks markdown docs in repo for humans and agents alike.

`wj` is the CLI. It manages the numbered `docs/` convention used across
projects: reference, review, task and bug documents with YAML frontmatter. It is
the single place the convention is defined, generated and checked. Humans and
agents both use it, and agents discover it by running it rather than reading
prose.

## Install

```
go install github.com/wjdp/wjdoc/cmd/wj@latest
```

## Quick start

```
wj init                          create docs/ and the about-doc
wj list                          table of every doc
wj new task Ship the loader      write up a task, then edit the file
wj set 7 status done             flip a frontmatter field
wj lint                          check the convention before committing
wj next                          what to pick up next
```

## Agent setup

```
wj setup
```

Installs the `wj-docs` Claude Code skill to `~/.claude/skills/wj-docs`, so an
agent picks up the `docs/` workflow (`wj next`, `wj new`, `wj set status`,
`wj lint`) without being told in prose.

## More

See `docs/001-Design.md` for the full design, or run `wj conventions`
(`wj schema` for the same rules as JSON).
