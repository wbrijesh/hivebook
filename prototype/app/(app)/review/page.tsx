"use client"

import { RiCheckDoubleLine } from "@remixicon/react"
import { toast } from "sonner"

import { ChromeActions } from "@/components/app/chrome"
import { PageHeading, PageShell, StatBar, Tag } from "@/components/app/page-kit"
import { Button } from "@/components/ui/button"
import { REVIEW_PROPOSALS, type ProposalKind } from "@/lib/mock-admin"
import { cn } from "@/lib/utils"

const KIND: Record<ProposalKind, { tone: "info" | "success" | "warning" | "muted"; label: string }> = {
  merge: { tone: "info", label: "Merge" },
  split: { tone: "info", label: "Split" },
  reclassify: { tone: "warning", label: "Reclassify" },
  "new-topic": { tone: "success", label: "New topic" },
  unfiled: { tone: "muted", label: "Unfiled" },
}

export default function ReviewPage() {
  const structural = REVIEW_PROPOSALS.filter((p) => p.kind !== "unfiled").length
  const unfiled = REVIEW_PROPOSALS.filter((p) => p.kind === "unfiled").length
  const avg = Math.round(
    (REVIEW_PROPOSALS.reduce((n, p) => n + p.confidence, 0) / REVIEW_PROPOSALS.length) * 100
  )

  return (
    <>
      <ChromeActions>
        <Button
          size="sm"
          variant="outline"
          onClick={() => toast("Approve all", { description: `${REVIEW_PROPOSALS.length} proposals applied to the book.` })}
        >
          <RiCheckDoubleLine className="size-3.5" />
          Approve all
        </Button>
      </ChromeActions>

      <PageShell>
        <PageHeading
          title="Review"
          description="Lower-confidence decisions the system parked for a human — structural changes and unfiled artifacts."
        />

        <StatBar
          items={[
            { label: "Open", value: `${REVIEW_PROPOSALS.length}`, sub: "proposals" },
            { label: "Structural", value: `${structural}`, sub: "splits, merges, topics" },
            { label: "Unfiled", value: `${unfiled}`, sub: "buckets" },
            { label: "Avg confidence", value: `${avg}%`, sub: "of the queue" },
          ]}
        />

        <div className="mt-7 space-y-3">
          {REVIEW_PROPOSALS.map((p) => {
            const k = KIND[p.kind]
            const pct = Math.round(p.confidence * 100)
            return (
              <div key={p.id} className="rounded-md border border-border bg-card p-3.5">
                <div className="flex items-center gap-2">
                  <Tag tone={k.tone}>{k.label}</Tag>
                  <span className="text-[14px] font-medium text-foreground">{p.title}</span>
                  <span className="ml-auto flex items-center gap-3 text-[12px] text-muted-foreground">
                    <span className="flex items-center gap-1.5">
                      <span className="h-1.5 w-12 overflow-hidden rounded-full bg-muted">
                        <span
                          className={cn(
                            "block h-full rounded-full",
                            pct >= 75 ? "bg-success" : "bg-warning"
                          )}
                          style={{ width: `${pct}%` }}
                        />
                      </span>
                      <span className="font-mono tabular-nums">{pct}%</span>
                    </span>
                    <span>{p.raisedAt}</span>
                  </span>
                </div>

                <p className="mt-1.5 text-[13px] leading-relaxed text-muted-foreground">{p.detail}</p>

                <div className="mt-3 flex items-center gap-2">
                  <Button size="xs" onClick={() => toast.success("Approved", { description: p.title })}>
                    Approve
                  </Button>
                  <Button size="xs" variant="outline" onClick={() => toast("Dismissed", { description: p.title })}>
                    Dismiss
                  </Button>
                  <span className="ml-auto text-[12px] text-muted-foreground">{p.target}</span>
                </div>
              </div>
            )
          })}
        </div>
      </PageShell>
    </>
  )
}
