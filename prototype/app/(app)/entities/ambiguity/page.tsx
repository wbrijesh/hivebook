"use client"

import { RiArrowRightLine } from "@remixicon/react"
import { toast } from "sonner"

import { PageHeading, PageShell, StatBar, Tag } from "@/components/app/page-kit"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { AMBIGUITY_ITEMS } from "@/lib/mock-admin"

export default function AmbiguityQueuePage() {
  const mentions = AMBIGUITY_ITEMS.reduce((n, i) => n + i.mentions, 0)

  return (
    <PageShell>
      <PageHeading
        title="Ambiguity queue"
        description="Where the system couldn't confidently resolve a reference. Resolving here teaches a prior for that source."
      />

      <StatBar
        items={[
          { label: "Open", value: `${AMBIGUITY_ITEMS.length}`, sub: "awaiting a human" },
          { label: "Affected", value: `${mentions}`, sub: "mentions" },
          { label: "Oldest", value: "2 days", sub: "in queue" },
        ]}
      />

      <div className="mt-7 space-y-3">
        {AMBIGUITY_ITEMS.map((item) => {
          const top = Math.max(...item.candidates.map((c) => c.score))
          return (
            <div key={item.id} className="overflow-hidden rounded-md border border-border bg-card">
              <div className="flex items-center gap-2 border-b border-border px-3.5 py-2.5">
                <span className="font-mono text-[13px] font-medium text-foreground">
                  “{item.surface}”
                </span>
                <span className="text-[12px] text-muted-foreground">{item.container}</span>
                <span className="ml-auto flex items-center gap-3 text-[12px] text-muted-foreground">
                  <span className="tabular-nums">{item.mentions} mentions</span>
                  <span>{item.raisedAt}</span>
                </span>
              </div>

              <div className="divide-y divide-border">
                {item.candidates.map((c) => {
                  const isTop = c.score === top
                  const pct = Math.round(c.score * 100)
                  return (
                    <div key={c.name} className="flex items-center gap-3 px-3.5 py-2">
                      <span className="min-w-0 flex-1 truncate text-[13px] text-foreground">
                        {c.name}
                        {isTop && (
                          <span className="ml-2 align-middle">
                            <Tag tone="info">likely</Tag>
                          </span>
                        )}
                      </span>
                      <span className="flex w-40 items-center gap-2">
                        <span className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
                          <span
                            className={cn(
                              "block h-full rounded-full",
                              isTop ? "bg-primary" : "bg-muted-foreground/40"
                            )}
                            style={{ width: `${pct}%` }}
                          />
                        </span>
                        <span className="w-8 text-right font-mono text-[12px] tabular-nums text-muted-foreground">
                          {pct}%
                        </span>
                      </span>
                      <Button
                        size="xs"
                        variant={isTop ? "default" : "outline"}
                        onClick={() =>
                          toast.success("Resolved", {
                            description: `“${item.surface}” → ${c.name}. Learned a prior for ${item.container}.`,
                          })
                        }
                      >
                        Assign
                      </Button>
                    </div>
                  )
                })}
              </div>

              <div className="flex items-center justify-between border-t border-border bg-muted/30 px-3.5 py-2">
                <span className="text-[12px] text-muted-foreground">
                  Resolving sets a prior for {item.container}.
                </span>
                <button
                  type="button"
                  onClick={() =>
                    toast("New entity", { description: `Create a canonical entity for “${item.surface}”.` })
                  }
                  className="inline-flex items-center gap-1 text-[12px] text-muted-foreground transition-colors hover:text-foreground"
                >
                  None of these — create entity
                  <RiArrowRightLine className="size-3.5" />
                </button>
              </div>
            </div>
          )
        })}
      </div>
    </PageShell>
  )
}
