"use client"

import { RiDownload2Line, RiVipCrownLine } from "@remixicon/react"
import { toast } from "sonner"

import { ChromeActions } from "@/components/app/chrome"
import {
  HeadRow,
  PageHeading,
  PageShell,
  Row,
  Section,
  StatBar,
  TableCard,
  Td,
  Th,
  fmtCount,
} from "@/components/app/page-kit"
import { Button } from "@/components/ui/button"
import { USAGE, USAGE_TREND } from "@/lib/mock-admin"
import { cn } from "@/lib/utils"

type Limit = { label: string; used: string; pct: number }

export default function UsagePage() {
  const tokenPct = USAGE.tokens.used / USAGE.tokens.limit
  const peak = Math.max(...USAGE_TREND.map((m) => m.tokens))

  const limits: Limit[] = [
    { label: "LLM tokens", used: `${fmtCount(USAGE.tokens.used)} / ${fmtCount(USAGE.tokens.limit)}`, pct: tokenPct },
    { label: "Storage", used: `${USAGE.storage.usedGb} / ${USAGE.storage.limitGb} GB`, pct: USAGE.storage.usedGb / USAGE.storage.limitGb },
    { label: "Seats", used: `${USAGE.seats.used} / ${USAGE.seats.limit}`, pct: USAGE.seats.used / USAGE.seats.limit },
    { label: "Connector syncs", used: `${fmtCount(USAGE.syncs.used)} / ${fmtCount(USAGE.syncs.limit)}`, pct: USAGE.syncs.used / USAGE.syncs.limit },
  ]

  return (
    <>
      <ChromeActions>
        <Button size="sm" variant="outline" onClick={() => toast("Preparing export", { description: "Usage report · CSV. We'll email a link." })}>
          <RiDownload2Line className="size-3.5" />
          Export
        </Button>
        <Button size="sm" variant="outline" onClick={() => toast("Plans", { description: "Compare Scale and Enterprise tiers." })}>
          <RiVipCrownLine className="size-3.5" />
          Manage plan
        </Button>
      </ChromeActions>

      <PageShell>
        <PageHeading
          title="Usage"
        description={`What this workspace is consuming against its contract — ${USAGE.plan} plan, ${USAGE.period}.`}
      />

      <StatBar
        items={[
          { label: "Plan", value: USAGE.plan, sub: USAGE.period },
          { label: "Tokens", value: fmtCount(USAGE.tokens.used), sub: `of ${fmtCount(USAGE.tokens.limit)}` },
          { label: "Storage", value: `${USAGE.storage.usedGb}GB`, sub: `of ${USAGE.storage.limitGb}GB` },
          { label: "Seats", value: `${USAGE.seats.used}`, sub: `of ${USAGE.seats.limit}` },
        ]}
      />

      <Section title="Token spend">
        <div className="rounded-md border border-border bg-card p-4">
          <div className="flex items-end gap-3">
            {USAGE_TREND.map((m, i) => {
              const last = i === USAGE_TREND.length - 1
              return (
                <div key={m.month} className="flex flex-1 flex-col items-center gap-1.5">
                  <span className="font-mono text-[11px] tabular-nums text-muted-foreground">
                    {m.tokens}M
                  </span>
                  <span className="flex h-28 w-full items-end">
                    <span
                      className={cn("w-full rounded-t-sm", last ? "bg-primary" : "bg-muted-foreground/25")}
                      style={{ height: `${Math.round((m.tokens / peak) * 100)}%` }}
                    />
                  </span>
                  <span className="text-[11px] text-muted-foreground">{m.month}</span>
                </div>
              )
            })}
          </div>
          <div className="mt-4 flex gap-6 border-t border-border pt-3 text-[12px]">
            <span className="text-muted-foreground">
              Query{" "}
              <span className="ml-1 font-mono tabular-nums text-foreground">
                {fmtCount(USAGE.tokens.query)}
              </span>
            </span>
            <span className="text-muted-foreground">
              Build{" "}
              <span className="ml-1 font-mono tabular-nums text-foreground">
                {fmtCount(USAGE.tokens.build)}
              </span>
            </span>
          </div>
        </div>
      </Section>

      <Section title="Limits" count={limits.length}>
        <TableCard>
          <HeadRow>
            <Th className="w-44">Resource</Th>
            <Th className="min-w-0 flex-1">Utilization</Th>
            <Th className="w-40 text-right">Used</Th>
          </HeadRow>
          {limits.map((l) => {
            const pct = Math.round(l.pct * 100)
            const high = pct >= 80
            return (
              <Row key={l.label}>
                <Td className="w-44 text-[13px] font-medium text-foreground">{l.label}</Td>
                <Td className="min-w-0 flex-1 gap-3">
                  <span className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
                    <span
                      className={cn("block h-full rounded-full", high ? "bg-warning" : "bg-success")}
                      style={{ width: `${pct}%` }}
                    />
                  </span>
                  <span className="w-9 text-right font-mono text-[12px] tabular-nums text-muted-foreground">
                    {pct}%
                  </span>
                </Td>
                <Td className="w-40 justify-end font-mono text-[12px] tabular-nums text-muted-foreground">
                  {l.used}
                </Td>
              </Row>
            )
          })}
        </TableCard>
      </Section>
      </PageShell>
    </>
  )
}
