"use client"

import * as React from "react"
import Link from "next/link"
import {
  RiAddLine,
  RiArrowRightSLine,
  RiCornerDownLeftLine,
  RiHistoryLine,
  RiSparkling2Line,
} from "@remixicon/react"
import { toast } from "sonner"

import { ChromeActions } from "@/components/app/chrome"
import { Section } from "@/components/app/page-kit"
import { Button } from "@/components/ui/button"
import { ConnectorIcon } from "@/components/brand/connector-icon"
import {
  entryStats,
  getArtifact,
  getCitation,
  getEntry,
  sourceByKind,
} from "@/lib/mock-book"
import { cn } from "@/lib/utils"

const SUGGESTIONS = [
  "What's our EU refund threshold?",
  "How does a Sev-1 incident escalate?",
  "Who signs off on large refunds?",
]

const RECENT = [
  "refund reason codes",
  "incident comms cadence",
  "US refund threshold over $600",
]

// Demo answer wired to the refunds knowledge so the surface feels real.
const ANSWER = {
  text: "Refunds are threshold-based. EU VAT refunds above €500 route to Finance for a second sign-off before they're issued; below the threshold, support can issue directly. [1] The threshold and a logged reason code became mandatory after the Q4 audit, [2] and a missed sign-off on an €820 refund is why it's now surfaced in the refund flow. [3]",
  citationIds: ["c1", "c2", "c3"],
  entries: ["s-eu-vat", "t-refunds"],
}

export default function AskPage() {
  const [query, setQuery] = React.useState("")
  const [asked, setAsked] = React.useState<string | null>(null)

  function submit(q: string) {
    const trimmed = q.trim()
    if (!trimmed) return
    setQuery(trimmed)
    setAsked(trimmed)
  }

  return (
    <div className="h-full overflow-y-auto">
      <ChromeActions>
        {asked && (
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              setAsked(null)
              setQuery("")
            }}
          >
            <RiAddLine className="size-3.5" />
            New question
          </Button>
        )}
        <Button size="sm" variant="outline" onClick={() => toast("History", { description: "Your recent questions and their answers." })}>
          <RiHistoryLine className="size-3.5" />
          History
        </Button>
      </ChromeActions>

      <div className="mx-auto max-w-[760px] px-8 py-7">
        <div className="flex items-center gap-2.5 rounded-lg border border-input bg-card px-3 py-2.5 focus-within:border-ring focus-within:ring-3 focus-within:ring-ring/30">
          <RiSparkling2Line className="size-4 shrink-0 text-primary" />
          <input
            autoFocus
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && submit(query)}
            placeholder="Ask anything about how your company works…"
            className="min-w-0 flex-1 bg-transparent text-[14px] text-foreground placeholder:text-muted-foreground focus:outline-none"
          />
          <button
            type="button"
            onClick={() => submit(query)}
            disabled={!query.trim()}
            className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[12px] text-muted-foreground transition-colors hover:text-foreground disabled:opacity-40"
          >
            <RiCornerDownLeftLine className="size-3.5" />
          </button>
        </div>

        {!asked ? (
          <>
            <div className="mt-3 flex flex-wrap gap-1.5">
              {SUGGESTIONS.map((s) => (
                <button
                  key={s}
                  type="button"
                  onClick={() => submit(s)}
                  className="rounded-full border border-border bg-card px-3 py-1 text-[12px] text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                >
                  {s}
                </button>
              ))}
            </div>

            <Section title="Recent" count={RECENT.length}>
              <div className="overflow-hidden rounded-md border border-border bg-card">
                {RECENT.map((r) => (
                  <button
                    key={r}
                    type="button"
                    onClick={() => submit(r)}
                    className="flex w-full items-center gap-2 border-b border-border px-3.5 py-2 text-left text-[13px] text-foreground transition-colors last:border-b-0 hover:bg-muted/50"
                  >
                    <span className="min-w-0 flex-1 truncate">{r}</span>
                    <RiArrowRightSLine className="size-4 shrink-0 text-muted-foreground" />
                  </button>
                ))}
              </div>
            </Section>
          </>
        ) : (
          <Answer />
        )}
      </div>
    </div>
  )
}

function Answer() {
  const parts = ANSWER.text.split(/(\[\d+\])/g)
  return (
    <div className="mt-6">
      <div className="mb-1.5 flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
        <RiSparkling2Line className="size-3.5 text-primary" />
        Answer
      </div>
      <article className="text-[14px] leading-relaxed text-foreground">
        <p>
          {parts.map((part, i) => {
            const m = part.match(/^\[(\d+)\]$/)
            if (!m) return <React.Fragment key={i}>{part}</React.Fragment>
            return (
              <a
                key={i}
                href={`#src-${m[1]}`}
                className="mx-0.5 inline-flex h-4 min-w-4 items-center justify-center rounded bg-muted px-1 align-super font-mono text-[10px] text-muted-foreground transition-colors hover:bg-primary/10 hover:text-primary"
              >
                {m[1]}
              </a>
            )
          })}
        </p>
      </article>

      <Section title="In the book" count={ANSWER.entries.length}>
        <div className="overflow-hidden rounded-md border border-border bg-card">
          {ANSWER.entries.map((id) => {
            const e = getEntry(id)
            if (!e) return null
            const s = entryStats(id)
            return (
              <Link
                key={id}
                href={`/book/${id}`}
                className="flex items-center gap-3 border-b border-border px-3.5 py-2 transition-colors last:border-b-0 hover:bg-muted/50"
              >
                <span className="min-w-0 flex-1 truncate text-[13px] text-foreground">
                  {e.title}
                </span>
                <span className="text-[12px] capitalize text-muted-foreground">{e.level}</span>
                <span className="font-mono text-[12px] tabular-nums text-muted-foreground">
                  {s.citations} cites
                </span>
                <RiArrowRightSLine className="size-4 shrink-0 text-muted-foreground" />
              </Link>
            )
          })}
        </div>
      </Section>

      <Section title="Sources" count={ANSWER.citationIds.length}>
        <div className="overflow-hidden rounded-md border border-border bg-card">
          {ANSWER.citationIds.map((cid, i) => {
            const cit = getCitation(cid)
            const art = cit && getArtifact(cit.artifactId)
            if (!cit || !art) return null
            const meta = sourceByKind(art.source)
            return (
              <div
                key={cid}
                id={`src-${i + 1}`}
                className="border-b border-border px-3.5 py-2.5 last:border-b-0"
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
                  <span className="ml-auto text-[11px] text-muted-foreground">{art.timestamp}</span>
                </div>
                <p className="mt-1 border-l-2 border-border pl-2 text-[12px] leading-relaxed text-muted-foreground">
                  {cit.quote}
                </p>
              </div>
            )
          })}
        </div>
      </Section>
    </div>
  )
}
