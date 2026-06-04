"use client"

import type { ReactNode } from "react"
import Link from "next/link"
import { RiRefreshLine } from "@remixicon/react"
import { toast } from "sonner"

import { ChromeActions } from "@/components/app/chrome"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  chapters,
  entryStats,
  recentEntries,
  systemStatus,
} from "@/lib/mock-book"
import { cn } from "@/lib/utils"

function fmt(n: number): string {
  return n >= 1000 ? `${(n / 1000).toFixed(1)}k` : `${n}`
}

export default function BookHomePage() {
  const sys = systemStatus()
  const recent = recentEntries()

  return (
    <div className="h-full overflow-y-auto">
      <ChromeActions>
        <Button size="sm" variant="outline" asChild>
          <Link href="/sources">Manage sources</Link>
        </Button>
        <Button
          size="sm"
          onClick={() =>
            toast("Rebuild queued", {
              description: `${sys.stale} stale ${sys.stale === 1 ? "entry" : "entries"} will refresh from the latest corpus.`,
            })
          }
        >
          <RiRefreshLine className="size-3.5" />
          Rebuild stale
          <Badge variant="warning">{sys.stale}</Badge>
        </Button>
      </ChromeActions>

      <div className="mx-auto max-w-[1100px] px-8 py-7">
        <h1 className="text-[15px] font-semibold tracking-tight text-foreground">
          Book
        </h1>
        <p className="mt-0.5 text-[13px] text-muted-foreground">
          The system's working model of the company — built and kept current from
          the corpus.
        </p>

        <StatStrip sys={sys} />

        {/* Chapters — the top-level structure as an object table */}
        <Section title="Chapters" count={chapters.length}>
          <div className="overflow-hidden rounded-md border border-border bg-card">
            <TableHead
              cols={["Chapter", "Topics", "Sub-topics", "Sources", "Coverage", "Last rebuilt"]}
            />
            {chapters.map((ch) => {
              const s = entryStats(ch.id)
              const building = ch.status === "building"
              return (
                <Row key={ch.id} href={`/book/${ch.id}`}>
                  <Cell className="min-w-0 flex-1">
                    <span className="flex items-center gap-2">
                      <StatusDot status={ch.status} />
                      <span className="truncate font-medium text-foreground">
                        {ch.title}
                      </span>
                    </span>
                  </Cell>
                  <Num>{building ? "—" : s.topics}</Num>
                  <Num>{building ? "—" : s.subtopics}</Num>
                  <Num>{building ? "—" : s.sources}</Num>
                  <Cell className="w-32">
                    {building ? (
                      <span className="text-[12px] text-muted-foreground">Building</span>
                    ) : (
                      <Coverage value={s.coverage} />
                    )}
                  </Cell>
                  <Cell className="w-28 justify-end text-[12px] text-muted-foreground">
                    {ch.lastRebuilt ?? "—"}
                  </Cell>
                </Row>
              )
            })}
          </div>
        </Section>

        {/* Recently rebuilt — the system's recent work */}
        <Section title="Recently rebuilt" count={recent.length}>
          <div className="overflow-hidden rounded-md border border-border bg-card">
            <TableHead cols={["Entry", "Level", "Sources", "Citations", "Status", "Rebuilt"]} />
            {recent.map((e) => {
              const s = entryStats(e.id)
              return (
                <Row key={e.id} href={`/book/${e.id}`}>
                  <Cell className="min-w-0 flex-1">
                    <span className="truncate text-foreground">{e.title}</span>
                  </Cell>
                  <Cell className="w-24 text-[12px] capitalize text-muted-foreground">
                    {e.level}
                  </Cell>
                  <Num>{s.sources}</Num>
                  <Num>{s.citations}</Num>
                  <Cell className="w-24">
                    <StatusTag status={e.status} />
                  </Cell>
                  <Cell className="w-28 justify-end text-[12px] text-muted-foreground">
                    {e.lastRebuilt ?? "—"}
                  </Cell>
                </Row>
              )
            })}
          </div>
        </Section>
      </div>
    </div>
  )
}

function StatStrip({ sys }: { sys: ReturnType<typeof systemStatus> }) {
  const stats = [
    { label: "Sources", value: `${sys.sources}`, sub: `${sys.sourcesHealthy} healthy` },
    { label: "Artifacts", value: fmt(sys.artifacts), sub: "ingested" },
    { label: "Entries", value: `${sys.entries}`, sub: `${sys.entities} entities` },
    { label: "Current", value: `${sys.current}`, sub: `${sys.stale} stale · ${sys.building} building` },
    { label: "Last sync", value: sys.lastSync, sub: "all sources" },
  ]
  return (
    <div className="mt-4 flex flex-wrap divide-x divide-border overflow-hidden rounded-md border border-border bg-card">
      {stats.map((s) => (
        <div key={s.label} className="min-w-[140px] flex-1 px-4 py-2.5">
          <div className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
            {s.label}
          </div>
          <div className="mt-0.5 text-[18px] font-semibold tabular-nums leading-none text-foreground">
            {s.value}
          </div>
          <div className="mt-1 text-[12px] text-muted-foreground">{s.sub}</div>
        </div>
      ))}
    </div>
  )
}

function Section({
  title,
  count,
  children,
}: {
  title: string
  count: number
  children: ReactNode
}) {
  return (
    <section className="mt-7">
      <div className="mb-2 flex items-baseline gap-2">
        <h2 className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
          {title}
        </h2>
        <span className="font-mono text-[11px] text-muted-foreground/70">{count}</span>
      </div>
      {children}
    </section>
  )
}

function TableHead({ cols }: { cols: string[] }) {
  return (
    <div className="flex items-center gap-3 border-b border-border bg-muted/40 px-3.5 py-1.5">
      {cols.map((c, i) => (
        <div
          key={c}
          className={cn(
            "text-[11px] font-medium uppercase tracking-wide text-muted-foreground",
            i === 0 ? "min-w-0 flex-1" : "shrink-0 text-right",
            i === 0 ? "" : colWidth(c)
          )}
        >
          {c}
        </div>
      ))}
    </div>
  )
}

function colWidth(label: string): string {
  if (label === "Coverage") return "w-32 text-left"
  if (label === "Level" || label === "Status") return "w-24 text-left"
  if (label === "Last rebuilt" || label === "Rebuilt") return "w-28"
  return "w-20"
}

function Row({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <Link
      href={href}
      className="flex items-center gap-3 border-b border-border px-3.5 py-2 text-[13px] transition-colors last:border-b-0 hover:bg-muted/50"
    >
      {children}
    </Link>
  )
}

function Cell({ className, children }: { className?: string; children: React.ReactNode }) {
  return <div className={cn("flex items-center", className)}>{children}</div>
}

function Num({ children }: { children: React.ReactNode }) {
  return (
    <div className="w-20 shrink-0 text-right font-mono text-[12px] tabular-nums text-foreground">
      {children}
    </div>
  )
}

function Coverage({ value }: { value: number }) {
  const pct = Math.round(value * 100)
  return (
    <span className="flex items-center gap-2">
      <span className="h-1.5 w-16 overflow-hidden rounded-full bg-muted">
        <span
          className={cn("block h-full rounded-full", pct >= 67 ? "bg-success" : "bg-warning")}
          style={{ width: `${pct}%` }}
        />
      </span>
      <span className="font-mono text-[12px] tabular-nums text-muted-foreground">{pct}%</span>
    </span>
  )
}

function StatusDot({ status }: { status: "current" | "stale" | "building" }) {
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

function StatusTag({ status }: { status: "current" | "stale" | "building" }) {
  const variant = { current: "success", stale: "warning", building: "muted" } as const
  const label = { current: "Current", stale: "Refreshing", building: "Building" }[status]
  return <Badge variant={variant[status]}>{label}</Badge>
}
