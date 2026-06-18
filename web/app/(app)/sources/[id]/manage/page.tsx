"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { useParams, useRouter } from "next/navigation"
import { useMutation, useQuery } from "@connectrpc/connect-query"
import { ConnectError } from "@connectrpc/connect"
import { keepPreviousData } from "@tanstack/react-query"
import { toast } from "sonner"

import { ConfirmDestructiveDialog } from "@/components/app/confirm-dialog"
import { ConnectorIcon } from "@/components/app/connector-icon"
import { PageShell } from "@/components/app/page-kit"
import { ProjectStatus } from "@/components/app/project-status"
import { SyncSchedule } from "@/components/app/sync-schedule"
import { PageHeader } from "@/components/patterns/page-header"
import { LineLoader } from "@/components/line-loader"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { useConnectionsStream } from "@/lib/api/use-connections-stream"
import { useProjectsStream } from "@/lib/api/use-projects-stream"
import { usePublishBreadcrumbLeaf } from "@/lib/breadcrumb-leaf"
import { RiDeleteBinLine, RiSearchLine } from "@remixicon/react"
import type { Project } from "@/lib/gen/hivebook/integration/v1/integration_pb"
import {
  disconnect,
  listConnectors,
  listProjects,
  setProjectSelection,
  updateSyncSchedule,
} from "@/lib/gen/hivebook/integration/v1/integration-IntegrationService_connectquery"

// Manage which projects a source syncs. Pure settings: pick the projects, click
// Save, done — saving persists the selection and marks the source onboarded
// server-side. It deliberately does NOT trigger a sync (the sync orchestrator owns
// that). The Sources list auto-opens this page once discovery finishes for a
// not-yet-onboarded source so the first thing the user does is choose what to sync.
export default function ManageConnectionPage() {
  const params = useParams<{ id: string }>()
  const connectors = useQuery(listConnectors, {})
  const { connections, error: streamError } = useConnectionsStream()

  // A just-created connection takes a beat to appear in the stream; hold the
  // skeleton briefly so onboarding doesn't flash "connection doesn't exist".
  const [graceElapsed, setGraceElapsed] = useState(false)
  useEffect(() => {
    const t = setTimeout(() => setGraceElapsed(true), 4000)
    return () => clearTimeout(t)
  }, [])

  const conn = connections?.find((c) => c.id === params.id)
  const connectorName =
    connectors.data?.connectors.find((c) => c.id === conn?.connectorId)?.name ??
    "Source"
  const title = conn?.account || connectorName

  // Publish the SOURCE name as the leaf — the breadcrumb nests "Manage" one level
  // under it (Sources / <source> / Manage), so the leaf is the source, not "Manage".
  usePublishBreadcrumbLeaf(
    conn
      ? `${connectorName}${conn.account ? ` · ${conn.account}` : ""}`
      : undefined
  )

  if (streamError) {
    return (
      <PageShell>
        <p className="text-[13px] text-muted-foreground">
          Couldn&rsquo;t load this source — the sync service is unreachable, or
          you may need to sign in again.{" "}
          <Link href="/sources" className="underline">
            Back to sources
          </Link>
          .
        </p>
      </PageShell>
    )
  }
  if (connections === null || (!conn && !graceElapsed)) {
    return <ManageSkeleton />
  }
  if (!conn) {
    return (
      <PageShell>
        <p className="text-[13px] text-muted-foreground">
          This connection no longer exists.{" "}
          <Link href="/sources" className="underline">
            Back to sources
          </Link>
          .
        </p>
      </PageShell>
    )
  }

  return (
    <ManagePanel
      connectionId={conn.id}
      connectorId={conn.connectorId}
      account={conn.account}
      connectorName={connectorName}
      title={title}
      onboarded={conn.onboarded}
      catalogDiscovered={!!conn.catalogDiscoveredAt}
      autoSyncEnabled={conn.autoSyncEnabled}
      syncIntervalSeconds={conn.syncIntervalSeconds}
    />
  )
}

// ManagePanel owns the selection state (mirrors ConnectorProjects' logic inline)
// so the page header can carry the Cancel/Save actions while the body holds the
// full-width project list. Split into its own component so the hooks below run
// unconditionally — the parent's early returns gate whether the connection exists.
function ManagePanel({
  connectionId,
  connectorId,
  account,
  connectorName,
  title,
  onboarded,
  catalogDiscovered,
  autoSyncEnabled,
  syncIntervalSeconds,
}: {
  connectionId: string
  connectorId: string
  account: string
  connectorName: string
  title: string
  onboarded: boolean
  catalogDiscovered: boolean
  autoSyncEnabled: boolean
  syncIntervalSeconds: number
}) {
  const router = useRouter()

  // Debounced project search → sent to the backend (ListProjects filters by name).
  // searchInput is what's typed; query is the debounced value we actually send.
  const [searchInput, setSearchInput] = useState("")
  const [query, setQuery] = useState("")
  useEffect(() => {
    const t = setTimeout(() => setQuery(searchInput.trim()), 250)
    return () => clearTimeout(t)
  }, [searchInput])
  const searching = query.length > 0

  // The FULL project set (unfiltered fetch as the seed, then the live stream) is the
  // selection truth: SetProjectSelection replaces the WHOLE selected list, so Save must
  // reckon over every project — not just what a search is showing. The stream also
  // carries live per-project status without clobbering the unsaved edits in `pending`.
  const discover = useQuery(listProjects, { connectionId })
  const streamed = useProjectsStream(connectionId)
  const allProjects = streamed ?? discover.data?.projects ?? null

  // A separate server-side search drives the TABLE while there's a query — keepPreviousData
  // and a fall-back to the full set so it doesn't flash while the debounced fetch lands.
  const searchResults = useQuery(
    listProjects,
    { connectionId, query },
    { enabled: searching, placeholderData: keepPreviousData }
  )
  const projects = searching
    ? (searchResults.data?.projects ?? allProjects)
    : allProjects

  // Unsaved per-project toggles, layered over the saved selection.
  const [pending, setPending] = useState<Record<string, boolean>>({})
  // Unsaved auto-sync schedule edits, layered over the connection's saved values.
  const [schedEnabled, setSchedEnabled] = useState(autoSyncEnabled)
  const [schedInterval, setSchedInterval] = useState(syncIntervalSeconds)
  const save = useMutation(setProjectSelection)
  const scheduleSave = useMutation(updateSyncSchedule)
  const saving = save.isPending || scheduleSave.isPending
  // Delete the source — this action lives on Manage, not the details page.
  const remove = useMutation(disconnect, {
    onSuccess: () => router.replace("/sources"),
    onError: (e) => toast.error(`Couldn't delete source: ${e.rawMessage}`),
  })

  // Projects with no estimated total (totalEstimate 0) can't be synced, so they're
  // disabled: selection, counts, and select-all all reckon over the selectable set.
  const isSelectable = (p: Project) => p.totalEstimate > 0
  const isSelected = (p: Project) => pending[p.id] ?? p.selected

  // `selectable` is the SHOWN set (drives the toolbar's select-all + count); dirty and
  // Save reckon over the FULL set so a change to a searched-away project isn't lost.
  const selectable = (projects ?? []).filter(isSelectable)
  const selectionDirty = (allProjects ?? []).some(
    (p) =>
      isSelectable(p) &&
      pending[p.id] !== undefined &&
      pending[p.id] !== p.selected
  )
  const scheduleDirty =
    schedEnabled !== autoSyncEnabled || schedInterval !== syncIntervalSeconds
  const dirty = selectionDirty || scheduleDirty
  const selectedCount = selectable.filter(isSelected).length
  const allSelected =
    selectable.length > 0 && selectedCount === selectable.length
  const someSelected = selectedCount > 0 && !allSelected

  function toggle(id: string, checked: boolean) {
    setPending((prev) => ({ ...prev, [id]: checked }))
  }

  function toggleAll() {
    const target = !allSelected
    setPending((prev) => ({
      ...prev,
      ...Object.fromEntries(selectable.map((p) => [p.id, target])),
    }))
  }

  async function onSave() {
    // Persist the project selection AND the auto-sync schedule in one Save. Saving the
    // selection marks the source onboarded server-side; on first-time onboarding we
    // always write it so onboarded flips even if the default (empty) selection is
    // unchanged. No sync is triggered here. The FIRST save (the "Continue" button)
    // hands the user to the details page; later saves keep them here to keep editing.
    const firstTime = !onboarded
    const selectedIds = (allProjects ?? [])
      .filter(isSelectable)
      .filter(isSelected)
      .map((p) => p.id)
    try {
      const ops: Promise<unknown>[] = []
      if (firstTime || selectionDirty)
        ops.push(save.mutateAsync({ connectionId, selectedIds }))
      if (scheduleDirty)
        ops.push(
          scheduleSave.mutateAsync({
            connectionId,
            enabled: schedEnabled,
            intervalSeconds: schedInterval,
          })
        )
      await Promise.all(ops)
    } catch (e) {
      toast.error(
        `Couldn't save: ${e instanceof ConnectError ? e.rawMessage : "please try again"}`
      )
      return
    }
    await discover.refetch()
    setPending({})
    toast.success("Saved")
    if (firstTime) router.push(`/sources/${connectionId}`)
  }

  // Heaviest projects lead — descending by estimated total, tiebreaking on how much
  // has synced, then name. Stable copy so we never mutate the streamed array.
  const ordered = [...(projects ?? [])].sort(
    (a, b) =>
      b.totalEstimate - a.totalEstimate ||
      b.artifactCount - a.artifactCount ||
      (a.name || a.id).localeCompare(b.name || b.id)
  )

  // Still discovering: no projects yet, or the catalog has never been discovered
  // (post-connect cold-start — the worker takes ~30-60s to list projects).
  const discovering =
    projects === null || (!catalogDiscovered && projects.length === 0)
  const empty = !discovering && (projects?.length ?? 0) === 0

  return (
    <PageShell>
      {/* Actions live in THIS page header (not the app chrome), right-aligned to
          the title — and only once discovery is done (nothing to save/cancel
          while we're still finding the projects). */}
      <PageHeader
        back="/sources"
        backLabel="Back to sources"
        icon={
          <ConnectorIcon
            connectorId={connectorId}
            className="size-6 shrink-0 text-foreground"
          />
        }
        title={
          onboarded ? `Manage ${title}` : `Set up ${title} as a data source`
        }
        subtitle={`Connected${account ? ` as ${account}` : ""} · ${connectorName}`}
        actions={
          !discovering ? (
            <>
              <Button
                variant="outline"
                size="sm"
                onClick={() =>
                  onboarded
                    ? router.push(`/sources/${connectionId}`)
                    : router.push("/sources")
                }
              >
                {onboarded ? "Cancel" : "I'll do this later"}
              </Button>
              <Button size="sm" onClick={onSave} disabled={!dirty || saving}>
                {saving ? "Saving…" : onboarded ? "Save" : "Continue"}
              </Button>
            </>
          ) : undefined
        }
      />

      {/* Schedule first — the per-source cadence, above the project list. */}
      {!discovering && (
        <>
          <p className="mt-6 text-[12px] font-medium text-muted-foreground">
            Schedule
          </p>
          <div className="mt-2">
            <SyncSchedule
              enabled={schedEnabled}
              intervalSeconds={schedInterval}
              onEnabledChange={setSchedEnabled}
              onIntervalChange={setSchedInterval}
            />
          </div>
        </>
      )}

      <p className="mt-6 text-[12px] font-medium text-muted-foreground">
        Projects to sync
      </p>
      <p className="mt-1 text-[13px] text-muted-foreground">
        Choose which projects to sync. You can change this anytime.
      </p>
      <div className="mt-2 overflow-hidden rounded-lg border border-border bg-card">
        {discovering ? (
          <div className="px-4 py-3">
            <p className="text-[12px] text-muted-foreground">
              Finding your projects… this can take up to a minute right after
              connecting.
            </p>
            <LineLoader className="mt-2.5" />
          </div>
        ) : empty && !searching ? (
          <div className="px-4 py-3 text-[12px] text-muted-foreground">
            No projects available for this account. If your repos live in an
            organization, make sure the GitHub App is installed on it with repo
            access.
          </div>
        ) : (
          <div>
            {/* Toolbar: tri-state select-all on the left, debounced search on the right. */}
            <div className="flex items-center justify-between gap-3 border-b border-border bg-muted/30 py-2 pr-2 pl-4">
              <div className="flex items-center gap-2.5 text-[12px] text-muted-foreground">
                <Checkbox
                  id="select-all-projects"
                  checked={
                    allSelected ? true : someSelected ? "indeterminate" : false
                  }
                  onCheckedChange={() => toggleAll()}
                  aria-label="Select all projects"
                />
                <label htmlFor="select-all-projects" className="cursor-pointer">
                  {selectedCount} of {selectable.length} selected
                </label>
              </div>
              <div className="relative w-56 max-w-[55%]">
                <RiSearchLine className="pointer-events-none absolute top-1/2 left-2 size-3.5 -translate-y-1/2 text-muted-foreground" />
                <Input
                  inputSize="sm"
                  value={searchInput}
                  onChange={(e) => setSearchInput(e.target.value)}
                  placeholder="Search projects…"
                  aria-label="Search projects"
                  className="bg-background pl-7"
                />
              </div>
            </div>
            {empty ? (
              <div className="px-4 py-6 text-center text-[13px] text-muted-foreground">
                No projects match &ldquo;{query}&rdquo;.
              </div>
            ) : (
              <ul className="max-h-[60vh] overflow-auto">
                {ordered.map((p) => {
                  const canSelect = isSelectable(p)
                  return (
                    <li key={p.id} className={cnRow(canSelect)}>
                      <Checkbox
                        id={`unit-${p.id}`}
                        checked={canSelect && isSelected(p)}
                        disabled={!canSelect}
                        onCheckedChange={(c) => toggle(p.id, c === true)}
                        aria-label={`Sync ${p.name || p.id}`}
                      />
                      <label
                        htmlFor={`unit-${p.id}`}
                        className={
                          canSelect
                            ? "min-w-0 flex-1 cursor-pointer truncate text-[13px] text-foreground"
                            : "min-w-0 flex-1 cursor-not-allowed truncate text-[13px] text-muted-foreground"
                        }
                      >
                        {p.name || p.id}
                      </label>
                      <span className="shrink-0 text-right">
                        <ProjectStatus project={p} />
                      </span>
                    </li>
                  )
                })}
              </ul>
            )}
          </div>
        )}
      </div>

      {/* Delete the source — lives here on Manage, not on the details page. */}
      {!discovering && (
        <>
          <p className="mt-8 text-[12px] font-medium text-muted-foreground">
            Danger zone
          </p>
          <div className="mt-2 flex items-center justify-between gap-4 rounded-lg border border-destructive/30 bg-card px-4 py-3">
            <div className="min-w-0">
              <p className="text-[13px] font-medium text-foreground">
                Delete source
              </p>
              <p className="text-[12px] text-muted-foreground">
                Disconnects {title} and schedules its synced files for deletion.
              </p>
            </div>
            <ConfirmDestructiveDialog
              trigger={
                <Button variant="destructive-outline" size="sm">
                  <RiDeleteBinLine data-icon="inline-start" />
                  Delete source
                </Button>
              }
              title={`Delete ${title}?`}
              description="This disconnects the source and schedules its synced files for deletion. You can reconnect later, but you’ll re-authorize."
              confirmWord={title}
              confirmLabel="Delete source"
              pending={remove.isPending}
              onConfirm={() => remove.mutate({ connectionId })}
            />
          </div>
        </>
      )}
    </PageShell>
  )
}

// Rows for un-estimable projects are visually muted and not clickable.
function cnRow(selectable: boolean): string {
  return selectable
    ? "flex items-center gap-3 px-4 py-2 hover:bg-muted/40"
    : "flex items-center gap-3 px-4 py-2 opacity-60 cursor-not-allowed"
}

function ManageSkeleton() {
  return (
    <PageShell>
      <div className="flex items-center gap-3">
        <Skeleton className="size-6 rounded-md" />
        <div className="space-y-1.5">
          <Skeleton className="h-4 w-40" />
          <Skeleton className="h-3 w-24" />
        </div>
      </div>
      <Skeleton className="mt-8 h-4 w-72" />
      <Skeleton className="mt-6 h-48 w-full rounded-lg" />
    </PageShell>
  )
}
