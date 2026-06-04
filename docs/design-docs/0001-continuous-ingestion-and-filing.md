---
design-id: 0001
status: accepted
date: 2026-05-24
authors:
  - Brijesh <google@brijesh.dev>
supersedes:
superseded-by:
related-adrs:
---

# 0001 — Continuous Ingestion and LLM-Driven Filing

## Context

Trenches ingests data continuously from many source systems and
must place every artifact into a hierarchical book index
(chapter → topic → sub-topic) per customer. The book index is not
bootstrapped per customer; the system designs and redesigns it
continuously as new content arrives. Both filing decisions (which
sub-topic an artifact belongs to) and structural decisions (new
sub-topics, splits, merges, renames) need to be made on every batch
of new content.

Per-artifact LLM calls are economically unviable at the volumes a
typical customer ingests. Embedding-only filing is too unreliable
for the structural choices the system needs to make and only
defers cost into the maintenance loop. The remaining option — and
the one we adopt — is **batched LLM calls** that handle filing,
structural decisions, and entity resolution together.

## Goals

- Place every ingested artifact in the right place in the book
  within a bounded time of arrival.
- Let the LLM continuously evolve the book structure to fit the
  customer's actual data shape.
- Keep per-tenant LLM cost predictable and proportional to
  artifact volume, not artifact count.
- Provide enough metadata for the maintenance loop to reason about
  the book without re-processing artifacts.

## Non-goals

- Real-time filing on a per-artifact basis. We accept a per-batch
  latency floor.
- Surfacing the filing decisions to end users. Users see the
  resulting book, not the pipeline.
- Allowing customers to provide their own filing rules. The LLM
  decides; admins influence via entity seed and access control,
  nothing else.

## Proposal

### Pipeline stages

```
Raw ingestion (continuous, no LLM)
        ↓
Segmentation (per-source rules)
        ↓
Batched filing + structural decisions + entity resolution
        (one LLM call per batch)
        ↓
Apply plan: file, mutate structure, mark summaries stale
        ↓
Update metadata
```

Entity resolution rides inside the same batched LLM call. See
`0003-entity-reconciliation.md` for the resolution-specific
mechanics.

### Batch trigger

A batch fires for a `(source, container)` pair when either:

- 15 minutes have elapsed since the last batch, **or**
- 20 artifacts have accumulated since the last batch.

Whichever comes first. Smaller batches at the end of a window are
acceptable.

These two numbers are **tunable defaults** drawn from runtime
configuration. The medium-term plan is to replace them with
per-container dynamic functions of source heat, arrival rate, and
artifact size. For v0.1 they are constants. See
`0004-tunable-defaults-to-dynamic-functions.md`.

### Pre-LLM retrieval (cheap, no LLM)

Before the LLM call, the system assembles context:

1. **Candidate sub-trees**: vector + BM25 retrieval over the book
   index returns the top-K candidate sub-topics per artifact. The
   candidates are deduplicated into a unified set with their
   parent topics and chapters expanded.
2. **Chapter table of contents**: full list of chapter and topic
   IDs and titles (no summaries). Cheap and bounded — even a
   mature book stays in a low-thousands token range.
3. **Recent entity context**: container priors and the recent
   resolution sliding window for the container (see `0003`).
4. **Entity store snapshot**: canonical names and aliases for the
   tenant's entities — also cheap.

This assembly involves only database reads and embedding lookups.
No LLM calls.

### The batched LLM call

Input:

- The batch of segmented artifacts
- Candidate sub-trees in full detail (sub-topic titles, recent
  evidence excerpts, current summaries if they exist)
- Chapter ToC (titles + IDs only)
- Recent entity context
- Entity store snapshot
- System prompt instructing the LLM to **prefer few structural
  changes** per batch, with examples of desirable batching

Output, structured:

```yaml
filings:
  - artifact_id: ...
    target_sub_topic_id: <existing-id> | NEW
    new_sub_topic_proposal:        # only if target = NEW
      title: ...
      parent_topic_id: <existing-id> | NEW
      new_topic_proposal: ...      # nested if needed
    confidence: 0.0–1.0
    resolved_entities: [...]
    reasoning: <short string>

structural_changes:
  - type: create_sub_topic | create_topic | split | merge | rename
    target_id: ...
    details: ...
    confidence: 0.0–1.0
    reason: <short string>
```

### Structural change budget

There is no hard numeric cap on structural changes per batch. The
system prompt instructs the LLM to minimize structural churn and
includes a small set of examples of how to consolidate related
changes. The LLM decides what is necessary. The maintenance loop
absorbs churn after the fact via consolidation.

### Apply plan

1. Each filing with confidence ≥ `HIGH_CONFIDENCE` (tunable
   default: 0.85) applies directly: the artifact is filed under
   the target sub-topic, creating any new sub-topic / topic on the
   path that the LLM proposed.
2. Filings with confidence below the threshold land in an
   **unfiled bucket** along with the candidate target. The
   maintenance loop reviews them.
3. Structural changes apply directly when confidence is high;
   lower-confidence proposals queue for the maintenance loop.
4. Every affected summary (sub-topic → parent topic → parent
   chapter) is marked stale per
   `0002-mark-and-defer-for-structural-changes.md`.
5. One audit log entry per filing and per structural change,
   correlated to the originating batch.

### Metadata updates per batch

For every touched sub-topic, the system updates incrementally:

- doc count delta
- contributor / author distribution delta
- source distribution delta
- entity overlap delta
- last-touched timestamp
- evidence content hash recomputed if evidence changed

Coherence score is **not** updated per batch; it requires
re-embedding work and is recomputed on a periodic sample by the
maintenance loop.

### Cost characteristics

For a source emitting 100 artifacts/hour, this pipeline yields
~4–5 LLM calls/hour rather than ~100. Each call is larger, but
per-token economics favor fewer larger calls. Net cost reduction
is one order of magnitude or more, with the LLM still making every
consequential decision.

### Cold start

Per customer, the book begins with a thin generic chapter skeleton
(5–8 broad chapters such as "operations", "engineering",
"customer", "finance", "people", "product"). Topics and sub-topics
start empty. The first batches naturally produce some volatile
structure; the maintenance loop consolidates over the first weeks
of ingestion.

### Idempotence and failure handling

- A batch is identified by its `(source, container, window)` tuple.
- The apply step is idempotent: re-running the same batch produces
  the same filings and structural changes (or is short-circuited
  by an existing batch record).
- If the LLM call fails, the batch is retried with exponential
  backoff. Raw artifacts are already in the corpus by this point
  so no data is lost.
- If applying the plan partially fails (e.g., a structural change
  conflicts with one applied by a concurrent batch), the
  conflicting change is dropped and re-queued for maintenance
  review.

## Alternatives considered

- **Embedding-only filing.** Cheaper per batch but unreliable for
  hierarchical placement and incapable of making structural
  decisions. Cost saving evaporates in the maintenance loop.
- **Per-artifact LLM call.** Highest quality, infeasible at scale.
- **Per-vertical fixed book templates.** Real organizations vary
  too much. Templates either shoehorn content into the wrong shape
  or proliferate into many bespoke templates.
- **Customer-bootstrapped book.** Reviewed and rejected — the
  customer doesn't see a guided book design; the system continuously
  designs it.

## Tradeoffs

- Per-batch latency: artifacts wait up to 15 minutes before being
  filed. Acceptable; the book is not a real-time activity feed.
- Batch failures take down a window's worth of progress.
  Idempotent retry compensates.
- LLM context size grows with book size. Mitigated by sending the
  chapter ToC only (titles + IDs), with full detail only for
  retrieved candidate sub-trees.

## Open questions

- Variable batch frequency per hot/cold source — deferred per
  `0004`.
- Exact value of `HIGH_CONFIDENCE` for direct apply — starts at
  0.85, will be revised from telemetry.
- Per-source segmentation rules — deserve a follow-up design doc.

## Decisions to be recorded as ADRs

- Batched LLM filing as the chosen filing strategy.
- 15 min / 20 items as the initial batch trigger.
- Thin generic chapter skeleton as the cold-start initial state.
