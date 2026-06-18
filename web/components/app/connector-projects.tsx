"use client"

import { useState } from "react"
import { useMutation, useQuery } from "@connectrpc/connect-query"
import { toast } from "sonner"
import { RiTimeLine } from "@remixicon/react"

import { AnimatedSpinner } from "@/components/animated-spinner"
import { Button } from "@/components/ui/button"
import { useProjectsStream } from "@/lib/api/use-projects-stream"
import type { Project } from "@/lib/gen/hivebook/integration/v1/integration_pb"
import {
  listProjects,
  setProjectSelection,
} from "@/lib/gen/hivebook/integration/v1/integration-IntegrationService_connectquery"

// ConnectorProjects is the per-connector settings panel: pick which projects to
// sync (default all) and watch each project's sync progress live. Selection is
// edited locally and saved explicitly; status streams in over WatchProjects
// without clobbering unsaved edits (standards/frontend/data-fetching).
export function ConnectorProjects({ connectionId }: { connectionId: string }) {
  // Discover the available projects live from the source (so you can choose what
  // to sync BEFORE syncing); the stream then carries live per-project status.
  const discover = useQuery(listProjects, { connectionId })
  const streamed = useProjectsStream(connectionId)
  const projects = streamed ?? discover.data?.projects ?? null
  // Unsaved per-project toggles, layered over the saved selection.
  const [pending, setPending] = useState<Record<string, boolean>>({})
  const save = useMutation(setProjectSelection, {
    // Refetch the saved selection before clearing the local edits — clearing first
    // would briefly fall back to the stale list (or a not-yet-pushed stream) and
    // visibly revert the checkboxes (data-fetching rule #3).
    onSuccess: async () => {
      await discover.refetch()
      setPending({})
    },
    onError: (e) => toast.error(`Couldn't save selection: ${e.rawMessage}`),
  })

  if (projects === null || (discover.isPending && projects.length === 0)) {
    return (
      <div className="flex items-center gap-2 px-4 py-3 text-[12px] text-muted-foreground">
        <AnimatedSpinner size="sm" tone="muted" />
        Discovering projects…
      </div>
    )
  }
  if (projects.length === 0) {
    return (
      <div className="px-4 py-3 text-[12px] text-muted-foreground">
        No projects available for this account. If your repos live in an
        organization, make sure the GitHub App is installed on it with repo
        access.
      </div>
    )
  }

  const isSelected = (p: Project) => pending[p.id] ?? p.selected
  const dirty = projects.some(
    (p) => pending[p.id] !== undefined && pending[p.id] !== p.selected
  )
  const selectedCount = projects.filter(isSelected).length
  const allSelected = projects.length > 0 && selectedCount === projects.length
  const someSelected = selectedCount > 0 && !allSelected
  const syncingCount = projects.filter((p) => p.status === "syncing").length
  const queuedCount = projects.filter((p) => p.status === "queued").length
  // "Active" spans both phases — queued (worker spinning up) and syncing (worker
  // running) — so the in-progress chrome shows continuously, with phase-aware copy.
  const anyActive = syncingCount > 0 || queuedCount > 0

  function toggle(id: string, checked: boolean) {
    setPending((prev) => ({ ...prev, [id]: checked }))
  }

  function toggleAll() {
    const target = !allSelected
    setPending(Object.fromEntries(projects!.map((p) => [p.id, target])))
  }

  function onSave() {
    const selectedIds = projects!.filter(isSelected).map((p) => p.id)
    save.mutate({ connectionId, selectedIds })
  }

  return (
    <div>
      {/* Toolbar header with a bottom border and a tri-state select-all. */}
      <div className="flex items-center justify-between border-b border-border bg-muted/30 px-4 py-2">
        <label className="flex items-center gap-2.5 text-[12px] text-muted-foreground">
          <input
            type="checkbox"
            checked={allSelected}
            ref={(el) => {
              if (el) el.indeterminate = someSelected
            }}
            onChange={toggleAll}
            className="size-3.5 accent-primary"
            aria-label="Select all projects"
          />
          {selectedCount} of {projects.length} selected
        </label>
        {dirty ? (
          <Button size="sm" onClick={onSave} disabled={save.isPending}>
            {save.isPending ? "Saving…" : "Save selection"}
          </Button>
        ) : anyActive ? (
          // One global phase here — "Preparing…" until a worker actually starts a
          // project (it spins up + discovers), then "Syncing…". Per-project state
          // (queued vs syncing) lives in the rows, not here.
          <span className="flex items-center gap-1.5 text-[12px] text-muted-foreground">
            <AnimatedSpinner size="sm" tone="primary" />
            {syncingCount > 0 ? "Syncing…" : "Preparing…"}
          </span>
        ) : null}
      </div>
      {/* Indeterminate sweep while a sync is in flight (queued or running) —
          activity, not completion (the item total isn't known up front). */}
      {anyActive && (
        <div className="h-0.5 w-full overflow-hidden bg-primary/10">
          <div className="h-full w-1/3 [animation:hb-indeterminate_1.2s_ease-in-out_infinite] rounded-full bg-primary" />
        </div>
      )}
      <ul className="max-h-72 overflow-auto p-2">
        {projects.map((p) => (
          <li
            key={p.id}
            className="flex items-center gap-3 rounded-md px-2 py-1.5"
          >
            <input
              type="checkbox"
              checked={isSelected(p)}
              onChange={(e) => toggle(p.id, e.target.checked)}
              className="size-3.5 accent-primary"
              aria-label={`Sync ${p.name || p.id}`}
            />
            <span className="min-w-0 flex-1 truncate text-[13px] text-foreground">
              {p.name || p.id}
            </span>
            <ProjectStatus project={p} />
          </li>
        ))}
      </ul>
    </div>
  )
}

function ProjectStatus({ project: p }: { project: Project }) {
  switch (p.status) {
    case "queued":
      // Waiting its turn in this connection's sync queue — not active yet, so no
      // spinner; a clock reads as "up next", not stalled.
      return (
        <span className="flex items-center gap-1.5 text-[12px] text-muted-foreground">
          <RiTimeLine className="size-3.5 shrink-0" />
          Queued
        </span>
      )
    case "syncing":
      // Live spinner; show the count as it climbs so progress is visible even
      // without a known total.
      return (
        <span className="flex items-center gap-1.5 text-[12px] text-primary">
          <AnimatedSpinner size="sm" tone="primary" />
          {p.artifactCount > 0 ? `${p.artifactCount} synced` : "Syncing…"}
        </span>
      )
    case "synced":
      return (
        <span className="text-[12px] text-muted-foreground">
          {p.artifactCount} synced
        </span>
      )
    case "error":
      return (
        <span className="text-[12px] text-destructive" title={p.lastError}>
          Error
        </span>
      )
    default:
      return <span className="text-[12px] text-muted-foreground">Idle</span>
  }
}
