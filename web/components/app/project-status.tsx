import { RiTimeLine } from "@remixicon/react"

import { AnimatedSpinner } from "@/components/animated-spinner"
import type { Project } from "@/lib/gen/hivebook/integration/v1/integration_pb"

// progressLabel reads "{synced} / {total}" once a total estimate is known (e.g.
// "0 / 958" before a sync starts), falling back to "{n} synced" / "0 items" when
// the source hasn't given an estimate yet.
export function progressLabel(p: Project): string {
  return p.totalEstimate > 0
    ? `${p.artifactCount.toLocaleString()} / ${p.totalEstimate.toLocaleString()}`
    : p.artifactCount > 0
      ? `${p.artifactCount.toLocaleString()} synced`
      : "0 items"
}

// ProjectStatus renders a project's live sync state + progress, right-aligned: a
// queued clock, a syncing spinner with synced/total, an error, or a settled count.
// Shared by the manage list (editable) and the details list (read-only).
export function ProjectStatus({ project: p }: { project: Project }) {
  switch (p.status) {
    case "queued":
      return (
        <span className="flex items-center justify-end gap-1.5 text-[12px] text-muted-foreground">
          <RiTimeLine className="size-3.5 shrink-0" />
          {progressLabel(p)}
        </span>
      )
    case "syncing":
      return (
        <span className="flex items-center justify-end gap-1.5 text-[12px] text-primary">
          <AnimatedSpinner size="sm" tone="primary" />
          {progressLabel(p)}
        </span>
      )
    case "error":
      return (
        <span className="text-[12px] text-destructive" title={p.lastError}>
          Error
        </span>
      )
    default:
      // Idle / synced — still show synced / total ("0 / 958") so the project's
      // size is legible before any sync runs.
      return (
        <span className="text-[12px] text-muted-foreground">
          {progressLabel(p)}
        </span>
      )
  }
}
