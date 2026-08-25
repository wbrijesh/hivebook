# Hivebook

Hivebook is a company brain: a structured, source-cited corpus of a company’s operational knowledge that people and AI agents can trust.

It captures knowledge from systems such as documents, source control, tickets, and conversations; retains the raw source material for provenance; and organizes it into a per-tenant book of summaries. The target book hierarchy is **chapter → topic → sub-topic**. Summaries remain traceable to their source evidence.

> **Status:** Hivebook is under active development. The repository contains the tenancy, onboarding, web, API, and integrations foundations. The wider corpus, retrieval, filing, and summary-lifecycle architecture is documented as the target system and is not all implemented yet. See [Architecture](#architecture-and-design) for the authoritative design record.

## Repository layout

| Path | Purpose |
| --- | --- |
| `api/` | Go API: Connect RPCs, tenant state, onboarding, ZITADEL JWT verification, and Postgres access. |
| `integrations/` | Go integrations service and Temporal worker: connector authorization, sync orchestration, raw capture, and object storage. |
| `web/` | Next.js web application. |
| `proto/` | Proto-first Connect contract; generated Go and TypeScript clients are committed. |
| `infra/` | Local Kubernetes manifests, Helm values, and provisioning scripts. |
| `docs/` | Antora documentation: product vision, architecture, design docs, ADRs, operations, and standards. |
| `.dagger/` | Hermetic CI and check pipeline. |
| `prototype/` | Opt-in mock-data UI prototype. |

## Local development

Hivebook runs locally in an OrbStack Kubernetes cluster. Go, Node, pnpm, Buf, and Antora run in containers; they do not need to be installed on the host.

### Prerequisites

- [OrbStack](https://orbstack.dev/) with Kubernetes enabled
- `just`, `gum`, `dagger`, `helm`, `mkcert`, and `jq`

On macOS:

```sh
brew install just gum dagger helm mkcert jq
```

### Set up the stack

```sh
just setup
```

The setup command is idempotent. It installs and trusts a local CA, creates the gitignored `infra/.env` file with local development secrets when absent, and deploys the stack.

Then inspect it:

```sh
just health
just urls
```

The main local surfaces are:

| Surface | URL |
| --- | --- |
| Web app | <https://app.hivebook.localhost> |
| Documentation | <https://docs.hivebook.localhost> |
| API | <https://api.hivebook.localhost> |
| ZITADEL | <https://id.hivebook.localhost> |
| Grafana | <https://grafana.hivebook.localhost> |

`just urls` prints the local-only login credentials. Do not reuse them outside local development.

### Day-to-day commands

```sh
just start                     # Start the stack
just stop                      # Stop workloads; preserve data
just status                    # Show pod readiness
just health                    # Check service and dependency health
just update web api            # Rebuild and roll out changed services
just check                     # Run the CI gate through Dagger
just proto                     # Regenerate Connect clients after proto changes
just gen                       # Regenerate sqlc query code
just reset-data                # Delete local application data (destructive)
```

Run `just` for the complete command list. After changing application code, use `just update <service...>` to rebuild and roll out the affected service. The buildable services are `api`, `web`, `integrations`, `docs`, and the opt-in `prototype`.

## Architecture and design

Hivebook separates source capture from interpretation:

1. **Connectors** authorize against external sources, sync incrementally, and capture raw artifacts.
2. **Raw corpus** stores each artifact verbatim in object storage with a Postgres manifest for provenance and deduplication.
3. **Canonicalization and retrieval** derive normalized, segmented, embedded content for hybrid search.
4. **Book index and summaries** organize knowledge into the three-level hierarchy and keep summaries source-cited.
5. **Enterprise controls** provide tenant isolation, authentication, access control, auditability, and residency controls across the system.

The documentation is the source of truth for this design:

- [Architecture overview](docs/modules/ROOT/pages/architecture/overview.adoc)
- [Ingestion architecture](docs/modules/ROOT/pages/architecture/ingestion.adoc)
- [v0.1 scope](docs/modules/ROOT/pages/roadmap/v0.1.adoc)
- [Operations: getting started](docs/modules/ROOT/pages/operations/getting-started.adoc)
- [Architecture decision records](docs/modules/ROOT/pages/adrs/index.adoc)

## Contributing

Read [the documentation style guide](docs/modules/ROOT/pages/style-guide.adoc) and the relevant standards before making a change. The project uses a proto-first API contract: edit `proto/`, run `just proto`, and commit generated clients when the contract changes.

`just check` is the required gate. It runs proto, web, API, and integrations checks in clean Dagger containers; CI runs the same pipeline.
