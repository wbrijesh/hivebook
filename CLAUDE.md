# CLAUDE.md

Hivebook is a **company brain** — a B2B system that pulls a company's scattered
operational knowledge into one structured, source-cited corpus that people and
AI agents can trust.

**This file is the map, not the manual.** It does not duplicate the docs — it
indexes them. Before working in an area, open the relevant page below and follow
the links between them (ADRs hold the rationale). The docs are the source of
truth.

## Documentation index

Docs are an Antora component under `docs/`; the pages live in
`docs/modules/ROOT/pages/`. Filenames are self-describing; open the page for the
area you're touching, and read `style-guide.adoc` (the house voice) and
`reference/glossary.adoc` (terms win ties) before contributing.

Regenerate this tree with `tree docs --gitignore` whenever docs are added or moved.

```
docs
├── antora.yml
├── Dockerfile
├── modules
│   └── ROOT
│       ├── nav.adoc
│       └── pages
│           ├── adrs
│           │   ├── 0000-template.adoc
│           │   ├── 0001-corpus-and-index-strata.adoc
│           │   ├── 0002-three-level-hierarchy.adoc
│           │   ├── 0003-hierarchy-invariant.adoc
│           │   ├── 0004-continuous-structure-design.adoc
│           │   ├── 0005-batched-llm-filing.adoc
│           │   ├── 0006-mark-and-defer.adoc
│           │   ├── 0007-hybrid-retrieval.adoc
│           │   ├── 0008-entity-reconciliation-in-filing.adoc
│           │   ├── 0009-chapter-level-access.adoc
│           │   ├── 0010-day-zero-enterprise-primitives.adoc
│           │   ├── 0011-constants-first-tunables.adoc
│           │   └── index.adoc
│           ├── architecture
│           │   ├── corpus-and-index.adoc
│           │   ├── enterprise-controls.adoc
│           │   ├── ingestion.adoc
│           │   ├── overview.adoc
│           │   ├── retrieval.adoc
│           │   ├── summary-lifecycle.adoc
│           │   └── tenancy-and-acl.adoc
│           ├── design
│           │   └── design-language.adoc
│           ├── design-docs
│           │   ├── 0000-template.adoc
│           │   ├── 0001-continuous-ingestion-and-filing.adoc
│           │   ├── 0002-mark-and-defer-for-structural-changes.adoc
│           │   ├── 0003-entity-reconciliation.adoc
│           │   ├── 0004-tunable-defaults-to-dynamic-functions.adoc
│           │   ├── 0005-tenancy-and-onboarding.adoc
│           │   └── index.adoc
│           ├── index.adoc
│           ├── operations
│           │   ├── api.adoc
│           │   ├── auth.adoc
│           │   └── infrastructure.adoc
│           ├── reference
│           │   ├── connector-catalog.adoc
│           │   ├── glossary.adoc
│           │   └── schemas
│           │       └── index.adoc
│           ├── roadmap
│           │   ├── milestones.adoc
│           │   └── v0.1.adoc
│           ├── standards
│           │   ├── docs
│           │   │   ├── adr-process.adoc
│           │   │   └── design-doc-process.adoc
│           │   ├── frontend
│           │   │   ├── component-patterns.adoc
│           │   │   ├── data-fetching.adoc
│           │   │   └── state-management.adoc
│           │   ├── go
│           │   │   ├── errors.adoc
│           │   │   ├── logging.adoc
│           │   │   ├── package-layout.adoc
│           │   │   └── testing.adoc
│           │   ├── index.adoc
│           │   └── infra
│           │       ├── labels.adoc
│           │       └── namespaces.adoc
│           ├── style-guide.adoc
│           └── vision
│               ├── audience.adoc
│               ├── principles.adoc
│               └── problem.adoc
└── supplemental-ui
    └── partials
        └── header-content.hbs
```

Quick map: `vision/` why · `architecture/` how-it's-built (links to ADRs) ·
`design-docs/` proposals · `adrs/` decisions · `standards/` binding conventions
(Go/frontend/infra/docs) · `design/` Apex UI language · `reference/` glossary &
catalogs · `roadmap/` · `operations/` running it.

## Standards are binding

Match the existing code. If no standard covers what you're doing, the
surrounding code _is_ the standard. To change a standard, edit its page in a PR
— never fork it locally. In particular:

- **One** logging style, **one** error shape, **one** response format
  (`standards/go/`). Don't add a second.
- Style the web only with semantic **Apex tokens**, never raw Tailwind palette
  classes (`design/design-language.adoc`).
- App workloads go in the `hivebook` namespace, not a new one
  (`standards/infra/namespaces.adoc`).
- Accepted ADRs are immutable — supersede, don't edit.

## How to work here (the quality bar)

- **Lead with the decision.** Terse, present-tense, no hedging — the house voice
  (`style-guide.adoc`) applies to code comments and docs alike.
- **Every load-bearing decision has a home** — an ADR (`adrs/`) or a design doc
  (`design-docs/`). Don't re-argue it in code or architecture docs; link to it.
- **Don't invent unknowns.** Mark them as explicit *Open questions*.
- **Match the surrounding code** — its naming, idiom, comment density.
- **Verify before claiming done.** `cd api && go test ./...` must pass; if you
  changed behavior, update the doc that describes it in the same change.

## Repo map

- `api/` — Go API: chi router, pgx/Postgres, `log/slog` JSON, ZITADEL JWT auth.
  Entrypoint `cmd/api/`, logic in `internal/{auth,database,server}`.
- `web/` — Next.js (App Router) + shadcn/ui + Apex tokens.
- `infra/` — Kubernetes manifests (OrbStack), one dir per component.
- `docs/` — the Antora docs this file indexes.
- `justfile` — the dev control panel (run `just`).

## Commands

- Dev cluster: `just start` · `just stop` · `just status` · `just health` · `just urls`
- API tests: `cd api && go test ./...`
- After changing code, rebuild + roll out: `just update <service...>` (e.g.
  `just update web api`) — builds the named services in parallel, zero-downtime.
- Full local install: `just install`

For logs and metrics use Grafana (`just urls`), not `just logs` — `just logs` is
only a quick live tail. Run `just` for the full panel and day-to-day guidance.

Everything runs on local OrbStack Kubernetes at `*.hivebook.localhost` over
mkcert TLS. Details: `operations/infrastructure.adoc`.
