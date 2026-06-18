"use client"

import { useEffect, useState } from "react"
import { useQuery } from "@connectrpc/connect-query"
import {
  RiCalendarLine,
  RiFocus3Line,
  RiPulseLine,
  RiUser3Line,
} from "@remixicon/react"
import { timestampDate } from "@bufbuild/protobuf/wkt"

import { PageHeading, PageShell } from "@/components/app/page-kit"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import type { AuditEvent } from "@/lib/gen/hivebook/tenant/v1/tenant_pb"
import { listAuditEvents } from "@/lib/gen/hivebook/tenant/v1/tenant-TenantService_connectquery"

const PAGE = 50

// Audit shows the workspace's trail of privileged actions, newest first
// (ADR-0010). Queryable and paginated; export is intentionally not here yet
// (roadmap v0.1).
export default function AuditPage() {
  const [offset, setOffset] = useState(0)
  const events = useQuery(listAuditEvents, { limit: PAGE, offset })
  const data = events.data
  const rows = data?.events ?? []
  const total = data?.total ?? 0
  const now = useNow()

  return (
    <PageShell>
      <PageHeading
        title="Audit log"
        description="Privileged actions in this workspace, newest first."
      />

      <div className="mt-6 overflow-hidden rounded-lg border border-border bg-card">
        <table className="w-full text-[13px]">
          <thead>
            <tr className="border-b border-border text-left text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
              <th className="px-4 py-2 font-medium">
                <span className="flex items-center gap-1.5">
                  <RiCalendarLine className="size-3.5 shrink-0" />
                  When
                </span>
              </th>
              <th className="px-3 py-2 font-medium">
                <span className="flex items-center gap-1.5">
                  <RiUser3Line className="size-3.5 shrink-0" />
                  Who
                </span>
              </th>
              <th className="px-3 py-2 font-medium">
                <span className="flex items-center gap-1.5">
                  <RiPulseLine className="size-3.5 shrink-0" />
                  Action
                </span>
              </th>
              <th className="px-4 py-2 font-medium">
                <span className="flex items-center gap-1.5">
                  <RiFocus3Line className="size-3.5 shrink-0" />
                  Target
                </span>
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {events.isPending &&
              Array.from({ length: 8 }).map((_, i) => (
                <tr key={i}>
                  <td className="px-4 py-2.5">
                    <Skeleton className="h-3.5 w-32" />
                  </td>
                  <td className="px-3 py-2.5">
                    <Skeleton className="h-3.5 w-40" />
                  </td>
                  <td className="px-3 py-2.5">
                    <Skeleton className="h-3.5 w-36" />
                  </td>
                  <td className="px-4 py-2.5">
                    <Skeleton className="h-3.5 w-28" />
                  </td>
                </tr>
              ))}
            {!events.isPending &&
              rows.map((e) => (
                <tr key={e.id} className="hover:bg-accent/40">
                  <td
                    className="px-4 py-2 whitespace-nowrap text-muted-foreground"
                    title={
                      e.occurredAt
                        ? timestampDate(e.occurredAt).toLocaleString()
                        : undefined
                    }
                  >
                    {e.occurredAt
                      ? timeAgo(timestampDate(e.occurredAt), now)
                      : "—"}
                  </td>
                  <td className="max-w-[14rem] truncate px-3 py-2 text-muted-foreground">
                    {e.actorEmail || e.actorId || "—"}
                  </td>
                  <td className="px-3 py-2 text-foreground">
                    {actionLabel(e.action)}
                  </td>
                  <td className="max-w-[16rem] truncate px-4 py-2 text-muted-foreground">
                    {e.target || "—"}
                  </td>
                </tr>
              ))}
          </tbody>
        </table>

        {!events.isPending && rows.length === 0 && (
          <div className="px-4 py-6 text-[13px] text-muted-foreground">
            No activity recorded yet.
          </div>
        )}
      </div>

      {total > PAGE && (
        <div className="mt-3 flex items-center justify-between text-[12px] text-muted-foreground">
          <span>
            {offset + 1}–{Math.min(offset + rows.length, total)} of {total}
          </span>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={offset === 0}
              onClick={() => setOffset(Math.max(0, offset - PAGE))}
            >
              Previous
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={offset + PAGE >= total}
              onClick={() => setOffset(offset + PAGE)}
            >
              Next
            </Button>
          </div>
        </div>
      )}
    </PageShell>
  )
}

// useNow returns the current epoch ms, refreshed each minute so the relative
// times stay accurate without a reload. The impure Date.now() lives only in the
// lazy initializer and the interval callback — never inline in render (purity).
function useNow(intervalMs = 60_000) {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), intervalMs)
    return () => clearInterval(id)
  }, [intervalMs])
  return now
}

// timeAgo renders a timestamp as a relative phrase ("17 minutes ago", "2 days
// ago") — the exact time stays available on the cell's hover title.
const RELATIVE = new Intl.RelativeTimeFormat("en", { numeric: "auto" })
const UNITS: [Intl.RelativeTimeFormatUnit, number][] = [
  ["year", 60 * 60 * 24 * 365],
  ["month", 60 * 60 * 24 * 30],
  ["week", 60 * 60 * 24 * 7],
  ["day", 60 * 60 * 24],
  ["hour", 60 * 60],
  ["minute", 60],
  ["second", 1],
]

function timeAgo(date: Date, now: number): string {
  const diffSeconds = Math.round((date.getTime() - now) / 1000) // negative = past
  const abs = Math.abs(diffSeconds)
  for (const [unit, secs] of UNITS) {
    if (abs >= secs || unit === "second") {
      return RELATIVE.format(Math.round(diffSeconds / secs), unit)
    }
  }
  return RELATIVE.format(0, "second")
}

// actionLabel maps the stable action codes the API records to readable text.
const ACTIONS: Record<string, string> = {
  "tenant.onboarded": "Completed onboarding",
  "tenant.updated": "Updated workspace settings",
  "source.connect_started": "Connected a source",
  "source.connected": "Connected a source",
  "source.disconnected": "Disconnected a source",
  "source.sync_triggered": "Triggered a sync",
  "source.sync_stopped": "Stopped a sync",
  "feature_flag.updated": "Changed a feature flag",
}

function actionLabel(action: AuditEvent["action"]): string {
  return ACTIONS[action] ?? action
}
