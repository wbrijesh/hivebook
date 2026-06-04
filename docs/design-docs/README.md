# Design Docs

A design doc proposes a specific component, feature, or significant
change before it is built. It is the place to argue about an
approach, not to record what was already decided.

## When to write one

Write a design doc when any of:

- the change touches multiple architecture layers
- the change involves a non-trivial tradeoff (perf vs. cost,
  simplicity vs. flexibility, build vs. integrate)
- there are reasonable alternative approaches worth considering
- the change has security, tenancy, or data-residency implications

If a change is small, mechanical, or obvious — just do it. Design
docs exist for the things that need discussion.

## Lifecycle

```
draft → in-review → accepted ──→ implemented ──→ superseded
              │
              └──→ rejected
```

- **draft** — author is writing
- **in-review** — open for comments
- **accepted** — approved to build
- **rejected** — closed without building; kept for the record
- **implemented** — built and shipped
- **superseded** — later design doc replaces this one (link to it)

## Outcome

An accepted design doc usually produces one or more ADRs that
capture the specific decisions made. The design doc explains *why*
the design is the design; the ADRs capture *what* was decided in a
form that survives editorial change.

## Conventions

- Numbered `NNNN-kebab-case-title.md` starting at `0001-`.
- Use the frontmatter from `0000-template.md`.
- Link to related ADRs when they are written.
- Keep prose dense — design docs are read, not skimmed.
