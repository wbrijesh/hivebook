// Mock company brain — the dataset behind every app surface. A 3-level book
// (chapter → topic → sub-topic) of cited summaries over raw artifacts, plus a
// thin generic chapter skeleton with some empty/building branches to show the
// cold-start state (per docs/design-docs/0001 and architecture/corpus-and-index).
// Stable IDs throughout; titles are mutable, IDs are not.

export type EntryLevel = "chapter" | "topic" | "subtopic"
export type EntryStatus = "current" | "stale" | "building"

export type BookEntry = {
  id: string
  level: EntryLevel
  parentId: string | null
  title: string
  status: EntryStatus
  /** Prose summary. Undefined while building / not yet generated. */
  summary?: string
  lastRebuilt?: string
  /** Sub-topic: source-span citation ids. Markers in `summary` are [1], [2]… */
  citationIds?: string[]
  /** Resolved entities this entry mentions (ids into ENTITIES). */
  entityIds?: string[]
}

export type SourceHealth = "synced" | "syncing" | "failed" | "auth-lapse"

export type Source = {
  id: string
  name: string
  kind: string
  color: string
  letter: string
  health: SourceHealth
  /** Raw artifacts ingested from this source. */
  artifacts: number
  /** Channels / spaces / projects watched. */
  containers: number
  lastSync: string
}

export type EntityType = "policy" | "team" | "role" | "event" | "system"
export type EntityStatus = "canonical" | "ambiguous"

export type Entity = {
  id: string
  name: string
  type: EntityType
  /** Known aliases folded into this canonical entity. */
  aliases: number
  /** Times referenced across the corpus. */
  mentions: number
  /** Distinct source systems it appears in. */
  sources: number
  status: EntityStatus
}

export type Artifact = {
  id: string
  source: string
  sourceLabel: string
  container: string
  author: string
  timestamp: string
  text: string
}

export type Citation = {
  id: string
  artifactId: string
  /** The exact cited span within the artifact. */
  quote: string
}

export const ARTIFACTS: Artifact[] = [
  {
    id: "a-slack-1",
    source: "slack",
    sourceLabel: "Slack",
    container: "#cs-refunds",
    author: "Dana Olsen",
    timestamp: "Mar 14, 2026 · 11:02",
    text: "For EU VAT refunds above €500 we always route to Finance for a second sign-off before issuing. Below that, support can issue directly. This came out of the Q4 audit — anything over the threshold needs the paper trail.",
  },
  {
    id: "a-notion-1",
    source: "notion",
    sourceLabel: "Notion",
    container: "Finance / Policies",
    author: "Priya Raman",
    timestamp: "Feb 2, 2026",
    text: "Refund policy v3. Thresholds: €500 (EU) / $600 (US). Above threshold requires Finance approval and a logged reason code. Reason codes live in the billing console.",
  },
  {
    id: "a-jira-1",
    source: "jira",
    sourceLabel: "Jira",
    container: "OPS",
    author: "Sam Okafor",
    timestamp: "Mar 9, 2026",
    text: "OPS-4471: customer issued €820 refund without Finance sign-off. Root cause: agent unaware of the EU threshold. Action: surface the threshold in the refund flow.",
  },
  {
    id: "a-slack-2",
    source: "slack",
    sourceLabel: "Slack",
    container: "#incidents",
    author: "Lee Carter",
    timestamp: "Mar 22, 2026 · 03:14",
    text: "Sev-1 page goes to the on-call SRE first. If unacked in 5 min it escalates to the secondary, then the EM. Open the incident channel before anything else so the timeline is captured.",
  },
  {
    id: "a-notion-2",
    source: "notion",
    sourceLabel: "Notion",
    container: "Engineering / Runbooks",
    author: "Priya Raman",
    timestamp: "Jan 18, 2026",
    text: "Incident severities. Sev-1: customer-facing outage. Escalation: on-call → secondary (5 min) → EM (10 min). Comms owner posts status every 15 min.",
  },
]

export const CITATIONS: Citation[] = [
  { id: "c1", artifactId: "a-slack-1", quote: "For EU VAT refunds above €500 we always route to Finance for a second sign-off before issuing." },
  { id: "c2", artifactId: "a-notion-1", quote: "Above threshold requires Finance approval and a logged reason code." },
  { id: "c3", artifactId: "a-jira-1", quote: "customer issued €820 refund without Finance sign-off. Root cause: agent unaware of the EU threshold." },
  { id: "c4", artifactId: "a-slack-2", quote: "Sev-1 page goes to the on-call SRE first. If unacked in 5 min it escalates to the secondary, then the EM." },
  { id: "c5", artifactId: "a-notion-2", quote: "Escalation: on-call → secondary (5 min) → EM (10 min). Comms owner posts status every 15 min." },
]

export const BOOK: BookEntry[] = [
  // Chapters — the thin generic skeleton.
  { id: "ch-ops", level: "chapter", parentId: null, title: "Operations", status: "current", summary: "How the company runs day to day — refunds, approvals, and operational policy. [1]", lastRebuilt: "2 days ago", citationIds: [] },
  { id: "ch-eng", level: "chapter", parentId: null, title: "Engineering", status: "current", summary: "How engineering builds and operates — incidents, runbooks, and release process.", lastRebuilt: "1 day ago" },
  { id: "ch-cust", level: "chapter", parentId: null, title: "Customer", status: "building" },
  { id: "ch-fin", level: "chapter", parentId: null, title: "Finance", status: "building" },
  { id: "ch-people", level: "chapter", parentId: null, title: "People", status: "building" },
  { id: "ch-product", level: "chapter", parentId: null, title: "Product", status: "building" },

  // Operations → topics
  { id: "t-refunds", level: "topic", parentId: "ch-ops", title: "Refunds & adjustments", status: "current", summary: "Refund handling is threshold-based: support issues small refunds directly; anything above the regional threshold needs Finance sign-off and a logged reason. [1][2]", lastRebuilt: "2 days ago" },
  { id: "t-approvals", level: "topic", parentId: "ch-ops", title: "Approvals & sign-offs", status: "building" },

  // Refunds → sub-topics
  { id: "s-eu-vat", level: "subtopic", parentId: "t-refunds", title: "EU VAT refunds over €500", status: "current", lastRebuilt: "Mar 15, 2026", summary: "EU VAT refunds above €500 are routed to Finance for a second sign-off before they're issued; below the threshold, support can issue directly. [1] The threshold and a logged reason code became mandatory after the Q4 audit, [2] and a missed sign-off on an €820 refund is why the threshold is now surfaced in the refund flow. [3]", citationIds: ["c1", "c2", "c3"], entityIds: ["e-eu-threshold", "e-finance", "e-q4-audit", "e-reason-code"] },
  { id: "s-us-refunds", level: "subtopic", parentId: "t-refunds", title: "US refunds over $600", status: "stale", lastRebuilt: "Jan 30, 2026", summary: "US refunds follow the same shape as EU with a $600 threshold; above it requires Finance approval and a reason code. [1]", citationIds: ["c2"], entityIds: ["e-finance", "e-reason-code"] },

  // Engineering → topics
  { id: "t-incidents", level: "topic", parentId: "ch-eng", title: "Incident response", status: "current", summary: "Incident response is severity-driven with a fixed escalation ladder and a comms cadence. [1]", lastRebuilt: "1 day ago" },

  // Incidents → sub-topics
  { id: "s-sev1", level: "subtopic", parentId: "t-incidents", title: "Sev-1 escalation path", status: "current", lastRebuilt: "Mar 23, 2026", summary: "A Sev-1 page goes to the on-call SRE first; if it's unacknowledged within 5 minutes it escalates to the secondary, then to the EM. [1] Severity definitions and the comms cadence (status every 15 minutes) live in the runbook. [2]", citationIds: ["c4", "c5"], entityIds: ["e-sev1", "e-oncall", "e-em"] },
]

// Connected source systems and their ingestion health — the gathering half of
// the system. Artifact counts roll up into the corpus the Book is built over.
export const SOURCES: Source[] = [
  { id: "src-slack", name: "Slack", kind: "slack", color: "#4A154B", letter: "S", health: "synced", artifacts: 18420, containers: 34, lastSync: "2 min ago" },
  { id: "src-notion", name: "Notion", kind: "notion", color: "#111111", letter: "N", health: "synced", artifacts: 1240, containers: 52, lastSync: "9 min ago" },
  { id: "src-jira", name: "Jira", kind: "jira", color: "#2684FF", letter: "J", health: "synced", artifacts: 6310, containers: 7, lastSync: "14 min ago" },
  { id: "src-github", name: "GitHub", kind: "github", color: "#1F2328", letter: "G", health: "syncing", artifacts: 9870, containers: 18, lastSync: "syncing…" },
  { id: "src-gdrive", name: "Google Drive", kind: "gdrive", color: "#1FA463", letter: "D", health: "auth-lapse", artifacts: 0, containers: 0, lastSync: "auth expired 2h ago" },
]

// Canonical entities resolved out of the corpus — the spine of cross-references
// the system maintains as it documents (per docs/design-docs/0003).
export const ENTITIES: Entity[] = [
  { id: "e-eu-threshold", name: "EU VAT threshold (€500)", type: "policy", aliases: 3, mentions: 14, sources: 3, status: "canonical" },
  { id: "e-finance", name: "Finance", type: "team", aliases: 2, mentions: 38, sources: 4, status: "canonical" },
  { id: "e-q4-audit", name: "Q4 2025 audit", type: "event", aliases: 1, mentions: 6, sources: 2, status: "canonical" },
  { id: "e-reason-code", name: "Reason code", type: "policy", aliases: 2, mentions: 11, sources: 2, status: "canonical" },
  { id: "e-sev1", name: "Sev-1", type: "policy", aliases: 4, mentions: 27, sources: 3, status: "canonical" },
  { id: "e-oncall", name: "On-call SRE", type: "role", aliases: 3, mentions: 19, sources: 2, status: "canonical" },
  { id: "e-em", name: "Engineering Manager", type: "role", aliases: 5, mentions: 9, sources: 2, status: "ambiguous" },
]

// — helpers —

export function getEntry(id: string): BookEntry | undefined {
  return BOOK.find((e) => e.id === id)
}

export function childrenOf(id: string | null): BookEntry[] {
  return BOOK.filter((e) => e.parentId === id)
}

export const chapters = childrenOf(null)

/** Breadcrumb chain root→entry. */
export function pathOf(id: string): BookEntry[] {
  const chain: BookEntry[] = []
  let cur = getEntry(id)
  while (cur) {
    chain.unshift(cur)
    cur = cur.parentId ? getEntry(cur.parentId) : undefined
  }
  return chain
}

export function getCitation(id: string): Citation | undefined {
  return CITATIONS.find((c) => c.id === id)
}

export function getArtifact(id: string): Artifact | undefined {
  return ARTIFACTS.find((a) => a.id === id)
}

export function getEntity(id: string): Entity | undefined {
  return ENTITIES.find((e) => e.id === id)
}

export function sourceByKind(kind: string): Source | undefined {
  return SOURCES.find((s) => s.kind === kind)
}

/** Recently-updated entries for the book home. */
export function recentEntries(): BookEntry[] {
  return BOOK.filter((e) => e.level !== "chapter" && e.summary).slice(0, 6)
}

// — surfaced state (the system showing its work) —

/** Every entry beneath `id`, any depth. */
export function descendantsOf(id: string): BookEntry[] {
  const out: BookEntry[] = []
  const stack = [...childrenOf(id)]
  while (stack.length) {
    const e = stack.pop()!
    out.push(e)
    stack.push(...childrenOf(e.id))
  }
  return out
}

/** Distinct raw artifacts an entry is built from (its own citations, or the
 *  union across the branch for a topic/chapter). */
export function artifactsOfEntry(id: string): Artifact[] {
  const e = getEntry(id)
  if (!e) return []
  const scope = e.level === "subtopic" ? [e] : [e, ...descendantsOf(id)]
  const ids = new Set<string>()
  for (const n of scope) {
    for (const cid of n.citationIds ?? []) {
      const c = getCitation(cid)
      if (c) ids.add(c.artifactId)
    }
  }
  return [...ids].map((a) => getArtifact(a)).filter(Boolean) as Artifact[]
}

/** Resolved entities for an entry (its own, or aggregated for a branch). */
export function entitiesOfEntry(id: string): Entity[] {
  const e = getEntry(id)
  if (!e) return []
  const ids = new Set<string>()
  const scope = e.level === "subtopic" ? [e] : [e, ...descendantsOf(id)]
  for (const n of scope) (n.entityIds ?? []).forEach((x) => ids.add(x))
  return [...ids].map((x) => getEntity(x)).filter(Boolean) as Entity[]
}

export type EntryStats = {
  topics: number
  subtopics: number
  sources: number
  citations: number
  entities: number
  current: number
  stale: number
  building: number
  /** Share of descendant entries that are current (0–1). */
  coverage: number
}

/** Rolled-up counts for an entry's subtree (the metadata a system surfaces). */
export function entryStats(id: string): EntryStats {
  const e = getEntry(id)
  if (!e) {
    return { topics: 0, subtopics: 0, sources: 0, citations: 0, entities: 0, current: 0, stale: 0, building: 0, coverage: 0 }
  }
  const scope = e.level === "subtopic" ? [e] : [e, ...descendantsOf(id)]
  const artifactIds = new Set<string>()
  const citationIds = new Set<string>()
  const entityIds = new Set<string>()
  for (const n of scope) {
    for (const cid of n.citationIds ?? []) {
      citationIds.add(cid)
      const c = getCitation(cid)
      if (c) artifactIds.add(c.artifactId)
    }
    ;(n.entityIds ?? []).forEach((x) => entityIds.add(x))
  }
  const counted = scope.filter((n) => n.level !== "chapter")
  const current = counted.filter((n) => n.status === "current").length
  const stale = counted.filter((n) => n.status === "stale").length
  const building = counted.filter((n) => n.status === "building").length
  return {
    topics: scope.filter((n) => n.level === "topic").length,
    subtopics: scope.filter((n) => n.level === "subtopic").length,
    sources: artifactIds.size,
    citations: citationIds.size,
    entities: entityIds.size,
    current,
    stale,
    building,
    coverage: counted.length ? current / counted.length : 0,
  }
}

export type Lineage = {
  artifacts: number
  sources: number
  lastRebuilt?: string
  queued: boolean
}

/** Where an entry's summary was built from, and whether a rebuild is pending. */
export function lineageOfEntry(id: string): Lineage {
  const e = getEntry(id)
  const arts = artifactsOfEntry(id)
  const systems = new Set(arts.map((a) => a.source))
  return {
    artifacts: arts.length,
    sources: systems.size,
    lastRebuilt: e?.lastRebuilt,
    queued: e?.status === "stale" || e?.status === "building",
  }
}

export type SystemStatus = {
  sources: number
  sourcesHealthy: number
  artifacts: number
  entries: number
  entities: number
  current: number
  stale: number
  building: number
  lastSync: string
}

/** Workspace-wide system state for the Book console header. */
export function systemStatus(): SystemStatus {
  const counted = BOOK.filter((e) => e.level !== "chapter")
  return {
    sources: SOURCES.length,
    sourcesHealthy: SOURCES.filter((s) => s.health === "synced" || s.health === "syncing").length,
    artifacts: SOURCES.reduce((n, s) => n + s.artifacts, 0),
    entries: BOOK.length,
    entities: ENTITIES.length,
    current: counted.filter((e) => e.status === "current").length,
    stale: counted.filter((e) => e.status === "stale").length,
    building: BOOK.filter((e) => e.status === "building").length,
    lastSync: "2 min ago",
  }
}
