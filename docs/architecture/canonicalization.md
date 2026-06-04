---
status: living
last-reviewed: 2026-05-24
---

# Summary Lifecycle

Every entry in the book index — chapter, topic, sub-topic — can
carry a summary. Summaries are LLM-generated and follow the
hierarchy invariant from `corpus-and-index.md`: sub-topic summaries
cite raw artifacts; topic summaries cite their child sub-topic
summaries; chapter summaries cite their child topic summaries.

Summaries are generated lazily and refreshed lazily. The system
does not eagerly summarize the whole book. The rules below define
when a summary is created or refreshed.

## When a summary is first generated

A sub-topic summary is generated when:

- The first query that touches the sub-topic requests it, or
- The maintenance loop decides the sub-topic crosses a generation
  threshold (enough artifacts filed, sufficient coherence,
  meaningful recency).

A topic summary is generated when its child sub-topics' summaries
have first been generated and either:

- A user query touches the topic and the summary doesn't yet
  exist, or
- The maintenance loop builds it because the topic is being shown
  in a browse path.

A chapter summary follows the same pattern, one level higher.

## When a summary is refreshed

A summary is **marked stale** by the mark-and-defer pattern (see
`design-docs/0002-mark-and-defer-for-structural-changes.md`)
whenever:

- A structural change touches the entry or its descendants.
- New evidence is filed under a descendant.
- Cited evidence is modified or deleted at the source.

A stale summary is **rebuilt** when:

- A user explicitly requests a fresh version.
- The first user read of a stale summary in a high-traffic entry
  triggers an async rebuild.
- The maintenance loop processes the stale queue.
- The summary's stale age exceeds the max-stale-age threshold (see
  `design-docs/0004-tunable-defaults-to-dynamic-functions.md`).

## Build process

For every build:

1. Gather the citations. Sub-topic builds gather source spans;
   topic and chapter builds gather child summaries.
2. Hand to the LLM with a structured prompt: produce a summary,
   cite every claim, flag contradictions or unknowns.
3. Validate that every claim has at least one citation. Reject and
   retry if not.
4. Store the new summary with `last_rebuilt = now`, citations, and
   a content hash of the cited evidence.

## Idempotent rebuilds

A rebuild is keyed by the content hash of the cited evidence. If
the hash matches the previously built summary's hash, the rebuild
short-circuits — no LLM call — and clears the stale flag.

This matters because mark-and-defer can mark a summary stale in
situations where the cited evidence didn't actually change (e.g.,
a sibling sub-topic was renamed). The idempotence check absorbs
that cost.

## Citations are non-negotiable

Every claim in a summary cites at least one piece of evidence. A
summary that fails citation validation is not written.

## Cost notes

- Aggressive caching: a summary is built once per evidence-hash
  and reused.
- Maintenance batching: rebuilds in the same maintenance pass share
  retrieval setup and are issued in parallel.
- Lazy default: most entries in a typical customer's book never
  carry a summary because they are never queried and never
  prioritized by maintenance.
