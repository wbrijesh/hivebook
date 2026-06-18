"use client"

import { useEffect, useState } from "react"
import { Clock } from "lucide-react"
import { timestampDate } from "@bufbuild/protobuf/wkt"
import type { Timestamp } from "@bufbuild/protobuf/wkt"

import { cn } from "@/lib/utils"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"

// RelativeTime renders a clock icon + a live relative label ("2 minutes ago"),
// with the exact local-timezone timestamp revealed on hover/focus. It's the one
// way a moment reads across the app — pass a proto Timestamp, a Date, or null.
// Null/undefined → a muted "Never" (no icon, no tooltip).
//
// The label re-renders on a 30s interval so "just now" ages into "1 minute ago"
// without a reload. We don't add a date library for this — formatRelative below
// is a compact hand-rolled formatter (date-fns isn't a dependency).
export function RelativeTime({
  value,
  className,
}: {
  value?: Timestamp | Date | null
  className?: string
}) {
  const date = toDate(value)
  // A stable epoch key for the effect — toDate() returns a fresh Date each render,
  // which would otherwise reset the interval on every parent re-render.
  const epoch = date ? date.getTime() : null

  // Tick every 30s so the relative label stays current while mounted.
  const [, setTick] = useState(0)
  useEffect(() => {
    if (epoch === null) return
    const id = setInterval(() => setTick((n) => n + 1), 30_000)
    return () => clearInterval(id)
  }, [epoch])

  if (!date) {
    return <span className={cn("text-muted-foreground", className)}>Never</span>
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          tabIndex={0}
          className={cn(
            "inline-flex items-center gap-1 rounded-sm outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
            className
          )}
        >
          <Clock className="size-3.5 shrink-0 text-muted-foreground" />
          {formatRelative(date)}
        </span>
      </TooltipTrigger>
      <TooltipContent>{formatExact(date)}</TooltipContent>
    </Tooltip>
  )
}

// toDate normalises the accepted inputs (proto Timestamp | Date | null) to a Date,
// or null when there's nothing to show.
function toDate(value?: Timestamp | Date | null): Date | null {
  if (!value) return null
  return value instanceof Date ? value : timestampDate(value)
}

// formatRelative renders a compact, human relative label ("just now", "2 minutes
// ago", "7 hours ago", "13 days ago"). Past-only by design — every timestamp we
// show (last synced, last checked) is in the past.
function formatRelative(date: Date): string {
  const seconds = Math.max(0, Math.round((Date.now() - date.getTime()) / 1000))
  if (seconds < 45) return "just now"

  const units: [limit: number, secs: number, name: string][] = [
    [60, 60, "minute"],
    [3600, 60, "minute"],
    [86_400, 3600, "hour"],
    [2_592_000, 86_400, "day"],
    [31_536_000, 2_592_000, "month"],
    [Infinity, 31_536_000, "year"],
  ]
  for (const [limit, secs, name] of units) {
    if (seconds < limit) {
      const n = Math.max(1, Math.floor(seconds / secs))
      return `${n} ${name}${n === 1 ? "" : "s"} ago`
    }
  }
  return "just now"
}

// formatExact renders the precise moment in the viewer's local timezone, named —
// e.g. "18 Jun 2026, 11:30:04 AM GMT+5:30".
function formatExact(date: Date): string {
  return new Intl.DateTimeFormat(undefined, {
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
    second: "2-digit",
    timeZoneName: "short",
    timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone,
  }).format(date)
}
