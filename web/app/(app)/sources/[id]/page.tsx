"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { useParams } from "next/navigation"
import { useMutation, useQuery } from "@connectrpc/connect-query"
import { toast } from "sonner"
import {
  RiEqualizerLine,
  RiFile2Line,
  RiLinkM,
  RiMore2Line,
  RiRefreshLine,
  RiStopLine,
} from "@remixicon/react"

import { AnimatedSpinner } from "@/components/animated-spinner"
import { connectionStatus } from "@/components/app/connection-status"
import { ConnectorIcon } from "@/components/app/connector-icon"
import { KeyValueList } from "@/components/app/key-value-list"
import { PageShell } from "@/components/app/page-kit"
import { ProjectStatus } from "@/components/app/project-status"
import { RelativeTime } from "@/components/app/relative-time"
import { PageHeader } from "@/components/patterns/page-header"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Skeleton } from "@/components/ui/skeleton"
import { useConnectionsStream } from "@/lib/api/use-connections-stream"
import { useProjectsStream } from "@/lib/api/use-projects-stream"
import { usePublishBreadcrumbLeaf } from "@/lib/breadcrumb-leaf"
import { ConnectionStatus } from "@/lib/gen/hivebook/integration/v1/integration_pb"
import {
  listConnectors,
  stopSync,
  triggerSync,
} from "@/lib/gen/hivebook/integration/v1/integration-IntegrationService_connectquery"

// One connection's detail view (by id — a connector type can have several): live
// status, metadata, and the read-only list of selected projects. Editing the
// selection, schedule, and deleting the source all live on the Manage page.
export default function ManageConnectionPage() {
  const params = useParams<{ id: string }>()
  const connectors = useQuery(listConnectors, {})
  const { connections, error: streamError } = useConnectionsStream()
  const projects = useProjectsStream(params.id)

  // e.rawMessage is the message without the "[code]" prefix that e.message carries —
  // the bracketed code is developer noise in a user-facing toast.
  const sync = useMutation(triggerSync, {
    onError: (e) => toast.error(`Couldn't start sync: ${e.rawMessage}`),
  })
  const stop = useMutation(stopSync, {
    onError: (e) => toast.error(`Couldn't stop sync: ${e.rawMessage}`),
  })

  // A just-created connection can take a beat to appear in the connections stream;
  // hold the skeleton briefly before declaring it gone, so a fresh connect doesn't
  // flash "this connection no longer exists".
  const [graceElapsed, setGraceElapsed] = useState(false)
  useEffect(() => {
    const t = setTimeout(() => setGraceElapsed(true), 4000)
    return () => clearTimeout(t)
  }, [])

  const conn = connections?.find((c) => c.id === params.id)
  const connectorName =
    connectors.data?.connectors.find((c) => c.id === conn?.connectorId)?.name ??
    "Source"

  // Publish the real connection name to the breadcrumb — the connector type plus
  // the account (e.g. "GitHub · brijesh-tai"), so the trail isn't a bare
  // "Connection". undefined while loading → the generic label shows briefly.
  usePublishBreadcrumbLeaf(
    conn
      ? `${connectorName}${conn.account ? ` · ${conn.account}` : ""}`
      : undefined
  )

  // A non-transient (auth) stream failure: surface it instead of freezing on the
  // skeleton forever (both streams have stopped reconnecting). Parity with the
  // Sources list's ErrorState.
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
  if (connections === null) {
    return <ManageSkeleton />
  }
  if (!conn) {
    // The stream may not carry a just-created connection yet — keep the skeleton
    // for a short grace before showing the hard "gone" message.
    if (!graceElapsed) {
      return <ManageSkeleton />
    }
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

  // Title is the account, not the connector type — two GitHub connections must
  // read differently (brijesh-tai vs wbrijesh).
  const title = conn.account || connectorName
  const status = connectionStatus(conn)
  // A sync is in flight when the connection is SYNCING — the server marks it the
  // moment we enqueue (MarkSyncing) and pushes it over WatchConnections; we don't
  // set it client-side. While it runs the action is Stop; STOPPING is the transient
  // settle, shown disabled; otherwise Sync now (disabled while the trigger is in
  // flight). The brief window after the trigger resolves but before the stream
  // pushes SYNCING is harmless — re-triggering is idempotent (discover is
  // unique-by-args, design-doc 0010).
  const syncing = conn.status === ConnectionStatus.SYNCING
  const stopping = conn.status === ConnectionStatus.STOPPING
  // Synced / estimated reckon over SELECTED projects only — unselected projects
  // don't sync, so they must not inflate the numerator or the "out of" total.
  const selectedProjects = projects?.filter((p) => p.selected) ?? []
  const selected = selectedProjects.length
  const totalItems = selectedProjects.reduce((n, p) => n + p.artifactCount, 0)
  const totalEstimate = selectedProjects.reduce(
    (n, p) => n + p.totalEstimate,
    0
  )
  // Read-only list of what's selected, heaviest first. Editing selection lives on
  // the manage page — the details page only shows what's being synced + progress.
  const selectedOrdered = [...selectedProjects].sort(
    (a, b) =>
      b.totalEstimate - a.totalEstimate ||
      b.artifactCount - a.artifactCount ||
      (a.name || a.id).localeCompare(b.name || b.id)
  )

  return (
    <PageShell>
      {/* Source-specific actions belong on the page, not the app chrome — the
          primary sync action and a ⋮ menu for the rest, as two separate buttons. */}
      <PageHeader
        back="/sources"
        backLabel="Back to sources"
        icon={
          <ConnectorIcon
            connectorId={conn.connectorId}
            className="size-6 shrink-0 text-foreground"
          />
        }
        title={title}
        subtitle={connectorName}
        actions={
          <>
            {syncing ? (
              // Sustained in-progress affordance — driven by the live stream, not
              // the mutation, so it no longer reverts the moment the trigger
              // resolves. The live "Items synced X / Y" lives in the KeyValueList.
              <Button
                variant="outline"
                size="sm"
                disabled={stop.isPending}
                onClick={() => stop.mutate({ connectionId: conn.id })}
              >
                {stop.isPending ? (
                  <AnimatedSpinner size="sm" />
                ) : (
                  <RiStopLine data-icon="inline-start" />
                )}
                {stop.isPending ? "Stopping…" : "Stop syncing"}
              </Button>
            ) : stopping ? (
              <Button variant="outline" size="sm" disabled>
                <AnimatedSpinner size="sm" />
                Stopping…
              </Button>
            ) : (
              <Button
                variant="outline"
                size="sm"
                disabled={sync.isPending}
                onClick={() => sync.mutate({ connectionId: conn.id })}
              >
                {sync.isPending ? (
                  <AnimatedSpinner size="sm" />
                ) : (
                  <RiRefreshLine data-icon="inline-start" />
                )}
                {sync.isPending ? "Syncing…" : "Sync now"}
              </Button>
            )}
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  variant="outline"
                  size="icon-sm"
                  aria-label="More actions"
                >
                  <RiMore2Line />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem asChild>
                  <Link href={`/files?connection=${conn.id}`}>
                    <RiFile2Line className="size-3.5" />
                    View files
                  </Link>
                </DropdownMenuItem>
                <DropdownMenuItem asChild>
                  <Link href={`/sources/${conn.id}/manage`}>
                    <RiEqualizerLine className="size-3.5" />
                    Manage
                  </Link>
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </>
        }
      />

      <KeyValueList
        className="mt-5"
        items={[
          {
            label: "Status",
            value: (
              <span
                className={
                  status.tone === "error"
                    ? "text-destructive"
                    : "text-foreground"
                }
              >
                {status.text}
              </span>
            ),
          },
          {
            label: "Last synced",
            value: <RelativeTime value={conn.lastSyncedAt} />,
          },
          {
            label: "Projects",
            value:
              projects === null
                ? "—"
                : `${selected} of ${projects.length} selected`,
          },
          {
            label: "Items synced",
            value:
              projects === null ? (
                "—"
              ) : totalItems > 0 ? (
                <Link
                  href={`/files?connection=${conn.id}`}
                  className="inline-flex items-center gap-1 border-b border-foreground/40 pb-px text-foreground transition-colors hover:border-primary hover:text-primary"
                >
                  <RiLinkM className="size-3.5" />
                  {totalEstimate > 0
                    ? `${totalItems.toLocaleString()} / ${totalEstimate.toLocaleString()}`
                    : totalItems.toLocaleString()}
                </Link>
              ) : totalEstimate > 0 ? (
                `0 / ${totalEstimate.toLocaleString()}`
              ) : (
                "0"
              ),
          },
          {
            label: "Auto-sync",
            value: conn.autoSyncEnabled
              ? `Every ${humanizeInterval(conn.syncIntervalSeconds)}`
              : "Off",
          },
        ]}
      />

      <p className="mt-6 text-[12px] font-medium text-muted-foreground">
        Selected projects
      </p>
      {/* Read-only: just what's selected + live progress. To change the selection,
          use Manage (in the ⋮ menu). */}
      <div className="mt-2 overflow-hidden rounded-lg border border-border bg-card">
        {projects === null ? (
          <div className="flex items-center gap-2 px-4 py-3 text-[12px] text-muted-foreground">
            <AnimatedSpinner size="sm" tone="muted" />
            Loading…
          </div>
        ) : selectedOrdered.length === 0 ? (
          <div className="px-4 py-3 text-[12px] text-muted-foreground">
            No projects selected.{" "}
            <Link
              href={`/sources/${conn.id}/manage`}
              className="text-foreground underline transition-colors hover:text-primary"
            >
              Choose what to sync
            </Link>
            .
          </div>
        ) : (
          <ul className="max-h-[60vh] overflow-auto">
            {selectedOrdered.map((p) => (
              <li key={p.id} className="flex items-center gap-3 px-4 py-2">
                <span className="min-w-0 flex-1 truncate text-[13px] text-foreground">
                  {p.name || p.id}
                </span>
                <span className="shrink-0 text-right">
                  <ProjectStatus project={p} />
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </PageShell>
  )
}

// humanizeInterval renders a sync interval (seconds) in the largest clean unit:
// 900 → "15 min", 3600 → "1 hour", 21600 → "6 hours", 86400 → "1 day".
function humanizeInterval(seconds: number): string {
  for (const [unit, label] of [
    [86400, "day"],
    [3600, "hour"],
    [60, "min"],
  ] as const) {
    if (seconds % unit === 0) {
      const n = seconds / unit
      return label === "min" ? `${n} min` : `${n} ${label}${n === 1 ? "" : "s"}`
    }
  }
  return `${Math.round(seconds / 60)} min`
}

function ManageSkeleton() {
  return (
    <PageShell>
      <div className="flex items-center gap-3">
        <Skeleton className="size-7 rounded-lg" />
        <Skeleton className="size-6 rounded-md" />
        <div className="space-y-1.5">
          <Skeleton className="h-4 w-40" />
          <Skeleton className="h-3 w-20" />
        </div>
      </div>
      <div className="mt-6 space-y-2.5">
        {Array.from({ length: 4 }).map((_, i) => (
          <Skeleton key={i} className="h-4 w-full max-w-sm" />
        ))}
      </div>
      <Skeleton className="mt-6 h-44 w-full rounded-lg" />
    </PageShell>
  )
}
