"use client"

import { Suspense, useEffect, useMemo, useRef } from "react"
import Link from "next/link"
import { useRouter, useSearchParams } from "next/navigation"
import { useQuery } from "@connectrpc/connect-query"

import { ChromeActions } from "@/components/app/chrome-actions"
import { connectionStatus } from "@/components/app/connection-status"
import { ConnectorIcon } from "@/components/app/connector-icon"
import { PageShell } from "@/components/app/page-kit"
import { PageHeader } from "@/components/patterns/page-header"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { LineLoader } from "@/components/line-loader"
import { useConnectionsStream } from "@/lib/api/use-connections-stream"
import type { Connection } from "@/lib/gen/hivebook/integration/v1/integration_pb"
import { listConnectors } from "@/lib/gen/hivebook/integration/v1/integration-IntegrationService_connectquery"

// The detail/manage route for a connection, by onboarded state: a not-yet-onboarded
// source goes to its manage page to pick projects; an onboarded one to its details.
function connectionHref(conn: Connection): string {
  return conn.onboarded ? `/sources/${conn.id}` : `/sources/${conn.id}/manage`
}

// Sources lists the connected systems with their live status. It is intentionally
// lean — connecting a new source happens in the Add-connector wizard (/sources/new),
// and managing one (projects, disconnect) on its detail page (/sources/[connector]).
//
// useSearchParams (the ?connected= auto-open marker) needs a Suspense boundary in the
// App Router — wrap the view, which also keeps the static shell painting immediately.
export default function SourcesPage() {
  return (
    <Suspense
      fallback={
        <PageShell>
          <PageHeader
            title="Sources"
            subtitle="The systems your knowledge is pulled from."
          />
        </PageShell>
      }
    >
      <SourcesView />
    </Suspense>
  )
}

function SourcesView() {
  const connectors = useQuery(listConnectors, {})
  const { connections, error } = useConnectionsStream()
  const router = useRouter()
  const searchParams = useSearchParams()
  const connectedId = searchParams.get("connected")

  const name = new Map(connectors.data?.connectors.map((c) => [c.id, c.name]))
  // Stable array identity so the auto-open effect's deps don't churn each render.
  const connected: Connection[] = useMemo(
    () => connections ?? [],
    [connections]
  )

  // Auto-open the just-connected source once discovery finishes: route to its manage
  // page (pick projects) when not yet onboarded, or details when already onboarded.
  // One-shot — a ref latches after the first fire so a re-render or a later stream
  // push can't loop or re-navigate; we only ever act on the ?connected= id.
  const autoOpened = useRef(false)
  useEffect(() => {
    if (autoOpened.current || !connectedId) return
    const conn = connected.find((c) => c.id === connectedId)
    // Wait until discovery completes (catalogDiscoveredAt set). If it never does, the
    // user simply stays on the list and can click the card.
    if (!conn || !conn.catalogDiscoveredAt) return
    autoOpened.current = true
    router.replace(connectionHref(conn))
  }, [connectedId, connected, router])

  // Distinguish a load failure from an empty workspace: the connections stream's
  // non-transient (auth) error, or a failed connector catalog. Without this a
  // failure would render as the "No sources connected" empty state.
  const failed = error !== null || connectors.isError
  const loading = !failed && (connections === null || connectors.isPending)

  return (
    <PageShell>
      {/* Page action lives in the chrome (context bar), not the page body. */}
      {connected.length > 0 && (
        <ChromeActions>
          <Button asChild size="sm">
            <Link href="/sources/new">Add source</Link>
          </Button>
        </ChromeActions>
      )}
      <PageHeader
        title="Sources"
        subtitle="The systems your knowledge is pulled from."
      />

      {failed ? (
        <ErrorState />
      ) : loading ? (
        <div className="mt-6 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <div
              key={i}
              className="flex items-center gap-3 rounded-lg border border-border bg-card px-4 py-4"
            >
              <Skeleton className="size-7 shrink-0 rounded-md" />
              <div className="flex-1 space-y-1.5">
                <Skeleton className="h-3.5 w-24" />
                <Skeleton className="h-3 w-32" />
              </div>
            </div>
          ))}
        </div>
      ) : connected.length === 0 ? (
        <EmptyState />
      ) : (
        <div className="mt-6 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {connected.map((conn) => (
            <SourceCard
              key={conn.id}
              conn={conn}
              connectorName={name.get(conn.connectorId) ?? conn.connectorId}
            />
          ))}
        </div>
      )}
    </PageShell>
  )
}

function SourceCard({
  conn,
  connectorName,
}: {
  conn: Connection
  connectorName: string
}) {
  const s = connectionStatus(conn)
  // Discovery is in flight while the catalog has never been discovered (the worker
  // cold-starts ~30-60s after connect). Surface it on the card so the user knows
  // we're working in the background, rather than showing a bare status.
  const discovering = !conn.catalogDiscoveredAt
  return (
    <Link
      href={connectionHref(conn)}
      className="flex items-center gap-3 rounded-lg border border-border bg-card px-4 py-4 transition-colors hover:bg-accent"
    >
      <ConnectorIcon
        connectorId={conn.connectorId}
        className="size-7 shrink-0 text-foreground"
      />
      <div className="min-w-0 flex-1">
        <p className="truncate text-[14px] font-medium text-foreground">
          {conn.account || connectorName}
        </p>
        {discovering ? (
          <div className="mt-1 space-y-1.5">
            <p className="truncate text-[12px] text-muted-foreground">
              Getting your data…
            </p>
            <LineLoader />
          </div>
        ) : (
          <p className="truncate text-[12px] text-muted-foreground">
            {connectorName} ·{" "}
            <span className={s.tone === "error" ? "text-destructive" : ""}>
              {s.text}
            </span>
          </p>
        )}
      </div>
    </Link>
  )
}

function ErrorState() {
  return (
    <div className="mt-6 flex flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-destructive/40 bg-card px-6 py-16 text-center">
      <p className="text-[14px] font-medium text-foreground">
        Couldn&rsquo;t load your sources
      </p>
      <p className="max-w-sm text-[13px] text-muted-foreground">
        Something went wrong reaching the sync service. Refresh to try again —
        if it persists, you may need to sign in again.
      </p>
    </div>
  )
}

function EmptyState() {
  return (
    <div className="mt-6 flex flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-border bg-card px-6 py-16 text-center">
      <p className="text-[14px] font-medium text-foreground">
        No sources connected
      </p>
      <p className="max-w-sm text-[13px] text-muted-foreground">
        Connect the systems where your knowledge lives — GitHub, Google Docs —
        and they&rsquo;re kept in sync.
      </p>
      <Button asChild size="sm" className="mt-1">
        <Link href="/sources/new">Add source</Link>
      </Button>
    </div>
  )
}
