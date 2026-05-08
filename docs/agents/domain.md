# Domain Docs

InstantRepo is single-context.

Read these before domain-sensitive work:

- `CONTEXT.md` at repo root
- `docs/adr/` when it exists

If a file does not exist, proceed silently. `/grill-with-docs` creates domain docs lazily when terms or decisions become clear.

## Layout

```text
/
├── CONTEXT.md
├── docs/
│   ├── agents/
│   └── adr/
└── internal/
```

## Vocabulary

Use terms from `CONTEXT.md` in issue titles, PRDs, test names, refactor plans, and implementation notes.

If needed term is missing, do not invent loose synonyms. Flag gap for `/grill-with-docs`.

## ADR Conflicts

If work contradicts an ADR, say so clearly before proposing change.
