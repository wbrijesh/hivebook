"use client"

import * as React from "react"
import Link from "next/link"
import { useParams } from "next/navigation"
import {
  RiArrowRightSLine,
  RiExternalLinkLine,
  RiLayoutRightLine,
  RiRefreshLine,
} from "@remixicon/react"
import { toast } from "sonner"

import {
  artifactsOfEntry,
  childrenOf,
  entitiesOfEntry,
  entryStats,
  getArtifact,
  getCitation,
  getEntry,
  lineageOfEntry,
  sourceByKind,
  type BookEntry,
  type Entity,
} from "@/lib/mock-book"
import { ChromeActions } from "@/components/app/chrome"
import { ConnectorIcon } from "@/components/brand/connector-icon"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

type Tab = "sources" | "entities" | "lineage"

export default function EntryPage() {
  const params = useParams<{ id: string }>()
  const entry = getEntry(params.id)
  const [activeCite, setActiveCite] = React.useState<string | null>(null)
  const [tab, setTab] = React.useState<Tab>("sources")
  const [requested, setRequested] = React.useState(false)
  const [inspectorOpen, setInspectorOpen] = React.useState(true)

  // Reset transient view state when navigating between entries.
  React.useEffect(() => {
    setActiveCite(null)
    setTab("sources")
    setRequested(false)
  }, [params.id])

  if (!entry) {
    return (
      <div className="grid h-full place-items-center px-6 text-center">
        <div>
          <p className="text-[14px] font-medium text-foreground">Entry not found</p>
          <p className="mt-1 text-[13px] text-muted-foreground">
            It may have been moved or merged.{" "}
            <Link href="/book" className="text-foreground underline-offset-2 hover:underline">
              Back to the book
            </Link>
            .
          </p>
        </div>
      </div>
    )
  }

  const stats = entryStats(entry.id)
  const kids = childrenOf(entry.id)
  const stale = entry.status === "stale" && !requested

  function openCitation(cid: string) {
    setActiveCite(cid)
    setTab("sources")
    setInspectorOpen(true)
  }

  function requestFresh() {
    setRequested(true)
    toast.success("Rebuild requested", {
      description: "This summary will refresh from the latest corpus shortly.",
    })
  }

  return (
    <div className="flex h-full">
      <ChromeActions>
        {entry.status === "building" ? (
          <span className="text-[12px] text-muted-foreground">Building…</span>
        ) : stale ? (
          <Button type="button" size="sm" variant="outline" onClick={requestFresh}>
            <RiRefreshLine className="size-3.5" />
            Request rebuild
          </Button>
        ) : null}
        <Button
          type="button"
          size="icon-sm"
          variant="ghost"
          aria-pressed={inspectorOpen}
          aria-label="Toggle inspector"
          onClick={() => setInspectorOpen((v) => !v)}
          className={cn(inspectorOpen && "bg-muted text-foreground")}
        >
          <RiLayoutRightLine className="size-4" />
        </Button>
      </ChromeActions>

      <div className="min-w-0 flex-1 overflow-y-auto">
        <div className="mx-auto max-w-[760px] px-9 py-8">
          <header className="mb-5">
            <h1 className="text-[19px] font-semibold tracking-tight text-foreground">
              {entry.title}
            </h1>
            <FactBand entry={entry} stats={stats} stale={stale} />
          </header>

          {entry.summary ? (
            <article className="text-[14px] leading-relaxed text-foreground">
              <Summary
                text={entry.summary}
                citationIds={entry.citationIds}
                onCite={openCitation}
                activeCitation={activeCite}
              />
            </article>
          ) : (
            <p className="rounded-md border border-dashed border-border px-4 py-8 text-center text-[13px] text-muted-foreground">
              {entry.level === "chapter"
                ? "This chapter is taking shape. As sources sync, the system files knowledge into topics here."
                : "No summary yet — it's generated once there's enough filed here."}
            </p>
          )}

          {entry.level !== "subtopic" && kids.length > 0 && (
            <section className="mt-7">
              <h2 className="mb-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                {entry.level === "chapter" ? "Topics" : "Sub-topics"} · {kids.length}
              </h2>
              <div className="overflow-hidden rounded-md border border-border bg-card">
                {kids.map((k) => {
                  const ks = entryStats(k.id)
                  return (
                    <Link
                      key={k.id}
                      href={`/book/${k.id}`}
                      className="flex items-center gap-3 border-b border-border px-3.5 py-2 text-[13px] transition-colors last:border-b-0 hover:bg-muted/50"
                    >
                      <StatusDot status={k.status} />
                      <span className="min-w-0 flex-1 truncate text-foreground">
                        {k.title}
                      </span>
                      <span className="shrink-0 font-mono text-[12px] tabular-nums text-muted-foreground">
                        {k.level === "subtopic" ? `${ks.citations} cites` : `${ks.subtopics} sub`}
                      </span>
                      <RiArrowRightSLine className="size-4 shrink-0 text-muted-foreground" />
                    </Link>
                  )
                })}
              </div>
            </section>
          )}
        </div>
      </div>

      {inspectorOpen && (
        <Inspector
          entry={entry}
          stats={stats}
          tab={tab}
          onTab={setTab}
          activeCitation={activeCite}
          onCite={setActiveCite}
        />
      )}
    </div>
  )
}

function FactBand({
  entry,
  stats,
  stale,
}: {
  entry: BookEntry
  stats: ReturnType<typeof entryStats>
  stale: boolean
}) {
  const facts: string[] = []
  if (entry.level !== "subtopic") {
    if (stats.topics) facts.push(`${stats.topics} topics`)
    facts.push(`${stats.subtopics} sub-topics`)
  }
  facts.push(`${stats.sources} sources`)
  if (entry.level === "subtopic") facts.push(`${stats.citations} citations`)
  facts.push(`${stats.entities} entities`)

  return (
    <div className="mt-2 flex flex-wrap items-center gap-x-2.5 gap-y-1 text-[12px] text-muted-foreground">
      <StatusTag status={stale ? "stale" : entry.status} />
      {entry.lastRebuilt && (
        <>
          <Sep />
          <span>rebuilt {entry.lastRebuilt}</span>
        </>
      )}
      {facts.map((f) => (
        <React.Fragment key={f}>
          <Sep />
          <span className="tabular-nums">{f}</span>
        </React.Fragment>
      ))}
    </div>
  )
}

function Sep() {
  return <span aria-hidden className="text-border">·</span>
}

// — center: summary prose, [n] markers → citation buttons —
function Summary({
  text,
  citationIds,
  onCite,
  activeCitation,
}: {
  text: string
  citationIds?: string[]
  onCite: (id: string) => void
  activeCitation: string | null
}) {
  const parts = text.split(/(\[\d+\])/g)
  return (
    <p>
      {parts.map((part, i) => {
        const m = part.match(/^\[(\d+)\]$/)
        if (!m) return <React.Fragment key={i}>{part}</React.Fragment>
        const cid = citationIds?.[Number(m[1]) - 1]
        if (!cid) return null
        return (
          <button
            key={i}
            type="button"
            onClick={() => onCite(cid)}
            className={cn(
              "mx-0.5 inline-flex h-4 min-w-4 items-center justify-center rounded px-1 align-super font-mono text-[10px] transition-colors",
              activeCitation === cid
                ? "bg-primary text-primary-foreground"
                : "bg-muted text-muted-foreground hover:bg-primary/10 hover:text-primary"
            )}
          >
            {m[1]}
          </button>
        )
      })}
    </p>
  )
}

// — right: persistent inspector —
function Inspector({
  entry,
  stats,
  tab,
  onTab,
  activeCitation,
  onCite,
}: {
  entry: BookEntry
  stats: ReturnType<typeof entryStats>
  tab: Tab
  onTab: (t: Tab) => void
  activeCitation: string | null
  onCite: (id: string | null) => void
}) {
  const tabs: { id: Tab; label: string; count: number }[] = [
    { id: "sources", label: "Sources", count: stats.sources },
    { id: "entities", label: "Entities", count: stats.entities },
    { id: "lineage", label: "Lineage", count: 0 },
  ]

  return (
    <aside className="flex w-[320px] shrink-0 flex-col overflow-hidden border-l border-border bg-background">
      <div className="flex shrink-0 items-center gap-1 border-b border-border px-2">
        {tabs.map((t) => (
          <button
            key={t.id}
            type="button"
            onClick={() => onTab(t.id)}
            className={cn(
              "relative flex items-center gap-1.5 px-2 py-2.5 text-[12px] font-medium transition-colors",
              tab === t.id
                ? "text-foreground"
                : "text-muted-foreground hover:text-foreground"
            )}
          >
            {t.label}
            {t.count > 0 && (
              <span className="font-mono text-[11px] text-muted-foreground/70">{t.count}</span>
            )}
            {tab === t.id && (
              <span className="absolute inset-x-1.5 -bottom-px h-0.5 rounded-full bg-primary" />
            )}
          </button>
        ))}
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto">
        {tab === "sources" && (
          <SourcesTab entry={entry} activeCitation={activeCitation} onCite={onCite} />
        )}
        {tab === "entities" && <EntitiesTab entry={entry} />}
        {tab === "lineage" && <LineageTab entry={entry} />}
      </div>
    </aside>
  )
}

function SourcesTab({
  entry,
  activeCitation,
  onCite,
}: {
  entry: BookEntry
  activeCitation: string | null
  onCite: (id: string | null) => void
}) {
  // Sub-topics show their citations 1:1 with the [n] markers; branches show the
  // distinct sources feeding everything beneath them.
  if (entry.level === "subtopic" && entry.citationIds?.length) {
    return (
      <div className="divide-y divide-border">
        {entry.citationIds.map((cid, i) => {
          const cit = getCitation(cid)
          const art = cit && getArtifact(cit.artifactId)
          if (!cit || !art) return null
          const active = activeCitation === cid
          const meta = sourceByKind(art.source)
          return (
            <button
              key={cid}
              type="button"
              onClick={() => onCite(active ? null : cid)}
              className={cn(
                "block w-full px-3 py-2.5 text-left transition-colors hover:bg-muted/50",
                active && "bg-brand-subtle/40"
              )}
            >
              <div className="flex items-center gap-2">
                <span className="font-mono text-[11px] text-muted-foreground">[{i + 1}]</span>
                <ConnectorIcon
                  color={meta?.color ?? "#6d6d6d"}
                  letter={meta?.letter ?? art.sourceLabel[0]}
                  size="sm"
                  className="size-4 rounded text-[9px]"
                />
                <span className="text-[12px] font-medium text-foreground">{art.sourceLabel}</span>
                <span className="truncate text-[11px] text-muted-foreground">{art.container}</span>
              </div>
              <p
                className={cn(
                  "mt-1.5 border-l-2 pl-2 text-[12px] leading-relaxed",
                  active ? "border-primary text-foreground" : "border-border text-muted-foreground"
                )}
              >
                {cit.quote}
              </p>
              <p className="mt-1 text-[11px] text-muted-foreground/80">
                {art.author} · {art.timestamp}
              </p>
            </button>
          )
        })}
      </div>
    )
  }

  const arts = artifactsOfEntry(entry.id)
  if (!arts.length) {
    return <Empty>No sources yet. They appear as the summary is built.</Empty>
  }
  return (
    <div className="divide-y divide-border">
      {arts.map((art) => {
        const meta = sourceByKind(art.source)
        return (
          <div key={art.id} className="flex items-start gap-2 px-3 py-2.5">
            <ConnectorIcon
              color={meta?.color ?? "#6d6d6d"}
              letter={meta?.letter ?? art.sourceLabel[0]}
              size="sm"
              className="size-5 rounded text-[10px]"
            />
            <div className="min-w-0">
              <div className="text-[12px] font-medium text-foreground">{art.sourceLabel}</div>
              <div className="truncate text-[11px] text-muted-foreground">
                {art.container} · {art.author}
              </div>
            </div>
          </div>
        )
      })}
    </div>
  )
}

function EntitiesTab({ entry }: { entry: BookEntry }) {
  const entities = entitiesOfEntry(entry.id)
  if (!entities.length) return <Empty>No entities resolved here yet.</Empty>
  return (
    <div className="divide-y divide-border">
      {entities.map((e) => (
        <div key={e.id} className="px-3 py-2.5">
          <div className="flex items-center gap-2">
            <span className="min-w-0 flex-1 truncate text-[13px] text-foreground">{e.name}</span>
            <EntityTypeTag type={e.type} />
          </div>
          <div className="mt-1 flex items-center gap-2.5 text-[11px] text-muted-foreground">
            <span className="tabular-nums">{e.mentions} mentions</span>
            <span className="tabular-nums">{e.sources} sources</span>
            <span className="tabular-nums">{e.aliases} aliases</span>
            {e.status === "ambiguous" && (
              <span className="ml-auto text-warning">ambiguous</span>
            )}
          </div>
        </div>
      ))}
    </div>
  )
}

function LineageTab({ entry }: { entry: BookEntry }) {
  const lin = lineageOfEntry(entry.id)
  const steps = [
    { label: "Sources", value: `${lin.sources} systems`, kind: "src" },
    { label: "Artifacts", value: `${lin.artifacts} cited spans`, kind: "art" },
    {
      label: "Summary",
      value: lin.lastRebuilt ? `rebuilt ${lin.lastRebuilt}` : "not yet built",
      kind: "sum",
    },
  ]
  return (
    <div className="px-3 py-3">
      <ol className="relative ml-1 border-l border-border pl-4">
        {steps.map((s) => (
          <li key={s.label} className="relative pb-4 last:pb-0">
            <span className="absolute -left-[21px] top-0.5 size-2.5 rounded-full border-2 border-background bg-muted-foreground/50" />
            <div className="text-[12px] font-medium text-foreground">{s.label}</div>
            <div className="text-[11px] text-muted-foreground">{s.value}</div>
          </li>
        ))}
      </ol>
      <div className="mt-1 rounded-md border border-border bg-muted/30 px-3 py-2 text-[12px]">
        <div className="flex items-center justify-between">
          <span className="text-muted-foreground">Rebuild</span>
          <span className={cn(lin.queued ? "text-warning" : "text-success")}>
            {lin.queued ? "queued" : "up to date"}
          </span>
        </div>
      </div>
      <button
        type="button"
        className="mt-3 inline-flex items-center gap-1.5 text-[12px] text-muted-foreground transition-colors hover:text-foreground"
      >
        <RiExternalLinkLine className="size-3.5" />
        Open build log
      </button>
    </div>
  )
}

function Empty({ children }: { children: React.ReactNode }) {
  return (
    <p className="px-3 py-8 text-center text-[12px] text-muted-foreground">{children}</p>
  )
}

function StatusDot({ status }: { status: BookEntry["status"] }) {
  return (
    <span
      className={cn(
        "size-1.5 shrink-0 rounded-full",
        status === "current"
          ? "bg-success"
          : status === "stale"
            ? "bg-warning"
            : "animate-pulse bg-muted-foreground/50"
      )}
    />
  )
}

function StatusTag({ status }: { status: BookEntry["status"] }) {
  const variant = { current: "success", stale: "warning", building: "muted" } as const
  const label = { current: "Current", stale: "Refreshing", building: "Building" }[status]
  return <Badge variant={variant[status]}>{label}</Badge>
}

function EntityTypeTag({ type }: { type: Entity["type"] }) {
  return <Badge variant="muted">{type}</Badge>
}
