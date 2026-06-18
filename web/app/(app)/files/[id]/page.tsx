"use client"

import { useMemo } from "react"
import { useParams } from "next/navigation"
import { useQuery } from "@connectrpc/connect-query"
import { RiExternalLinkLine } from "@remixicon/react"
import { timestampDate } from "@bufbuild/protobuf/wkt"

import { ChromeActions } from "@/components/app/chrome-actions"
import { JsonViewer } from "@/components/app/json-viewer"
import { KeyValueList } from "@/components/app/key-value-list"
import { PageShell } from "@/components/app/page-kit"
import { PageHeader } from "@/components/patterns/page-header"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { usePublishBreadcrumbLeaf } from "@/lib/breadcrumb-leaf"
import {
  getArtifactContent,
  listArtifacts,
} from "@/lib/gen/hivebook/integration/v1/integration-IntegrationService_connectquery"

// File detail: the synced item's content rendered in-app (JSON as a collapsible
// tree, anything else as text) — never a direct object-storage URL; the bytes come
// through our own authenticated API.
export default function FileDetailPage() {
  const { id } = useParams<{ id: string }>()
  // Metadata comes from the (cached) list; content from its own RPC.
  const artifacts = useQuery(listArtifacts, { limit: 200, offset: 0 })
  const content = useQuery(getArtifactContent, { id })
  const artifact = artifacts.data?.artifacts.find((a) => a.id === id)

  const decoded = useMemo(() => {
    const bytes = content.data?.data
    if (!bytes) return null
    const text = new TextDecoder().decode(bytes)
    try {
      return { json: JSON.parse(text) as unknown, text }
    } catch {
      return { json: undefined, text }
    }
  }, [content.data])

  const title = artifact?.externalId || artifact?.sourceNativeKind || "File"

  // Publish the file's real name to the breadcrumb instead of a generic "File".
  usePublishBreadcrumbLeaf(artifact ? title : undefined)

  return (
    <PageShell>
      {/* Page action lives in the chrome (context bar), not the page body. */}
      {artifact?.sourceUrl && (
        <ChromeActions>
          <Button asChild variant="outline" size="sm">
            <a href={artifact.sourceUrl} target="_blank" rel="noreferrer">
              <RiExternalLinkLine data-icon="inline-start" />
              Open original
            </a>
          </Button>
        </ChromeActions>
      )}

      <PageHeader
        back="/files"
        backLabel="Back to files"
        title={title}
        subtitle={
          artifact
            ? `${artifact.connectorId}${artifact.containerName ? ` · ${artifact.containerName}` : ""}`
            : undefined
        }
      />

      {artifact && (
        <KeyValueList
          className="mt-5"
          items={[
            { label: "Source", value: artifact.connectorId },
            { label: "Project", value: artifact.containerName || "—" },
            { label: "Kind", value: artifact.sourceNativeKind || "—" },
            {
              label: "Synced",
              value: artifact.ingestedAt
                ? timestampDate(artifact.ingestedAt).toLocaleString()
                : "—",
            },
          ]}
        />
      )}

      <p className="mt-6 text-[12px] font-medium text-muted-foreground">
        Content
      </p>
      <div className="mt-2 rounded-lg border border-border bg-card p-3">
        {content.isPending ? (
          <ContentSkeleton />
        ) : content.error ? (
          <p className="px-1 py-4 text-[13px] text-destructive">
            Couldn&rsquo;t load this file: {content.error.message}.
          </p>
        ) : decoded?.json !== undefined ? (
          <JsonViewer value={decoded.json} />
        ) : (
          <pre className="overflow-auto font-mono text-[12.5px] leading-relaxed text-foreground">
            {decoded?.text || "Empty file."}
          </pre>
        )}
      </div>
    </PageShell>
  )
}

function ContentSkeleton() {
  return (
    <div className="space-y-2 p-1">
      {[90, 70, 80, 55, 75, 60].map((w, i) => (
        <Skeleton key={i} className="h-3.5" style={{ width: `${w}%` }} />
      ))}
    </div>
  )
}
