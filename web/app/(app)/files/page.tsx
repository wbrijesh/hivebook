"use client"

import { Suspense, useMemo, useState } from "react"
import Link from "next/link"
import { useRouter, useSearchParams } from "next/navigation"
import { useQuery } from "@connectrpc/connect-query"
import {
  RiArrowDownSLine,
  RiCalendarLine,
  RiCloseLine,
  RiExternalLinkLine,
  RiFolder3Line,
  RiPlugLine,
  RiText,
} from "@remixicon/react"
import { timestampDate } from "@bufbuild/protobuf/wkt"

import { ConnectorIcon } from "@/components/app/connector-icon"
import { PageShell } from "@/components/app/page-kit"
import { PageHeader } from "@/components/patterns/page-header"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Skeleton } from "@/components/ui/skeleton"
import {
  listArtifacts,
  listConnections,
} from "@/lib/gen/hivebook/integration/v1/integration-IntegrationService_connectquery"

// Files lists the raw items Hivebook has synced. Viewing one opens its own page
// (bytes served through our API, never a direct storage URL). A `?connection=<id>`
// param scopes it to one source — the source→files deep link.
export default function FilesPage() {
  return (
    <Suspense
      fallback={
        <PageShell>
          <PageHeader title="Files" />
        </PageShell>
      }
    >
      <FilesView />
    </Suspense>
  )
}

function FilesView() {
  const router = useRouter()
  const params = useSearchParams()
  const connectionId = params.get("connection") ?? ""

  const artifacts = useQuery(listArtifacts, {
    limit: 200,
    offset: 0,
    connectionId,
  })
  const items = useMemo(() => artifacts.data?.artifacts ?? [], [artifacts.data])
  const [project, setProject] = useState(params.get("project") ?? "all")

  // Connections resolve each file's real source — its account ("brijesh-tai"),
  // not just the platform ("github") — and back the filter chip.
  const connections = useQuery(listConnections, {})
  const connById = useMemo(
    () => new Map((connections.data?.connections ?? []).map((c) => [c.id, c])),
    [connections.data]
  )
  const source = connectionId ? connById.get(connectionId) : undefined

  const projects = useMemo(
    () =>
      Array.from(
        new Set(items.map((a) => a.containerName).filter(Boolean))
      ).sort(),
    [items]
  )
  const shown =
    project === "all" ? items : items.filter((a) => a.containerName === project)

  return (
    <PageShell>
      <PageHeader
        title="Files"
        subtitle="Everything synced from your connected sources."
      />

      <div className="mt-6 flex flex-wrap items-center gap-2">
        {connectionId && (
          // Active source filter — the deep link from a connector's detail page.
          <span className="inline-flex items-center gap-1.5 rounded-lg border border-border bg-card py-1 pr-1 pl-2 text-[12px]">
            {source && (
              <ConnectorIcon
                connectorId={source.connectorId}
                className="size-3.5 shrink-0 text-muted-foreground"
              />
            )}
            <span className="text-foreground">
              {source?.account || source?.connectorId || "Source"}
            </span>
            <Button
              asChild
              size="icon-xs"
              variant="ghost"
              aria-label="Clear source filter"
            >
              <Link href="/files">
                <RiCloseLine />
              </Link>
            </Button>
          </span>
        )}

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" size="sm">
              {project === "all" ? "All projects" : project}
              <RiArrowDownSLine data-icon="inline-end" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            align="start"
            className="max-h-72 w-56 overflow-auto"
          >
            <DropdownMenuRadioGroup value={project} onValueChange={setProject}>
              <DropdownMenuRadioItem value="all">
                All projects
              </DropdownMenuRadioItem>
              {projects.map((p) => (
                <DropdownMenuRadioItem key={p} value={p}>
                  {p}
                </DropdownMenuRadioItem>
              ))}
            </DropdownMenuRadioGroup>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <div className="mt-3 overflow-hidden rounded-lg border border-border bg-card">
        <table className="w-full text-[13px]">
          <thead>
            <tr className="border-b border-border text-left text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
              <th className="px-4 py-2 font-medium">
                <span className="flex items-center gap-1.5">
                  <RiText className="size-3.5 shrink-0" />
                  Name
                </span>
              </th>
              <th className="px-3 py-2 font-medium">
                <span className="flex items-center gap-1.5">
                  <RiPlugLine className="size-3.5 shrink-0" />
                  Source
                </span>
              </th>
              <th className="px-3 py-2 font-medium">
                <span className="flex items-center gap-1.5">
                  <RiFolder3Line className="size-3.5 shrink-0" />
                  Project
                </span>
              </th>
              <th className="px-3 py-2 font-medium">
                <span className="flex items-center gap-1.5">
                  <RiCalendarLine className="size-3.5 shrink-0" />
                  Synced
                </span>
              </th>
              <th className="px-4 py-2 text-right font-medium">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {artifacts.isPending
              ? Array.from({ length: 6 }).map((_, i) => (
                  <tr key={i}>
                    <td className="px-4 py-2.5">
                      <Skeleton className="h-3.5 w-48" />
                    </td>
                    <td className="px-3 py-2.5">
                      <Skeleton className="h-3.5 w-16" />
                    </td>
                    <td className="px-3 py-2.5">
                      <Skeleton className="h-3.5 w-24" />
                    </td>
                    <td className="px-3 py-2.5">
                      <Skeleton className="h-3.5 w-28" />
                    </td>
                    <td className="px-4 py-2.5">
                      <Skeleton className="ml-auto h-3.5 w-10" />
                    </td>
                  </tr>
                ))
              : shown.map((a) => (
                  <tr
                    key={a.id}
                    onClick={() => router.push(`/files/${a.id}`)}
                    className="cursor-pointer hover:bg-accent/40"
                  >
                    <td className="max-w-0 px-4 py-2">
                      <span className="block truncate font-medium text-foreground">
                        {a.externalId || a.sourceNativeKind}
                      </span>
                      <span className="block truncate text-[11px] text-muted-foreground">
                        {a.sourceNativeKind}
                      </span>
                    </td>
                    <td className="max-w-[12rem] px-3 py-2">
                      <SourceCell
                        connectorId={
                          connById.get(a.connectionId)?.connectorId ??
                          a.connectorId
                        }
                        account={connById.get(a.connectionId)?.account ?? ""}
                      />
                    </td>
                    <td className="max-w-[14rem] truncate px-3 py-2 text-muted-foreground">
                      {a.containerName || "—"}
                    </td>
                    <td className="px-3 py-2 whitespace-nowrap text-muted-foreground">
                      {a.ingestedAt
                        ? timestampDate(a.ingestedAt).toLocaleString()
                        : "—"}
                    </td>
                    <td className="px-4 py-2 text-right whitespace-nowrap">
                      {a.sourceUrl && (
                        <Button asChild size="icon-xs" variant="ghost">
                          <a
                            href={a.sourceUrl}
                            target="_blank"
                            rel="noreferrer"
                            aria-label="Open original"
                            onClick={(e) => e.stopPropagation()}
                          >
                            <RiExternalLinkLine />
                          </a>
                        </Button>
                      )}
                    </td>
                  </tr>
                ))}
          </tbody>
        </table>

        {!artifacts.isPending && shown.length === 0 && (
          <div className="px-4 py-6 text-[13px] text-muted-foreground">
            {items.length === 0
              ? connectionId
                ? "No files from this source yet."
                : "No files yet. Connect a source and sync to see synced items here."
              : "No files in this project."}
          </div>
        )}
      </div>

      {artifacts.data && artifacts.data.total > items.length && (
        <p className="mt-3 text-[12px] text-muted-foreground">
          Showing {items.length} of {artifacts.data.total}.
        </p>
      )}
    </PageShell>
  )
}

// SourceCell names the file's actual source — the account (e.g. "brijesh-tai"),
// with the platform icon — so two GitHub connections read differently. Falls back
// to the platform id when the connection is gone or still loading.
function SourceCell({
  connectorId,
  account,
}: {
  connectorId: string
  account: string
}) {
  return (
    <span className="flex items-center gap-1.5 text-foreground">
      <ConnectorIcon
        connectorId={connectorId}
        className="size-4 shrink-0 text-muted-foreground"
      />
      <span className="truncate">{account || connectorId}</span>
    </span>
  )
}
