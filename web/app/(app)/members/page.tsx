"use client"

import { useQuery } from "@connectrpc/connect-query"
import { RiMailLine, RiShieldUserLine, RiUser3Line } from "@remixicon/react"

import { PageHeading, PageShell } from "@/components/app/page-kit"
import { Skeleton } from "@/components/ui/skeleton"
import type { Member } from "@/lib/gen/hivebook/tenant/v1/tenant_pb"
import { listMembers } from "@/lib/gen/hivebook/tenant/v1/tenant-TenantService_connectquery"

// Members lists the people in the workspace, read live from the identity provider
// (ZITADEL is the directory, ADR-0012). Read-only for now — inviting and role
// changes happen in the identity provider until the role model lands.
export default function MembersPage() {
  const members = useQuery(listMembers, {})
  const data = members.data
  const rows = data?.members ?? []

  return (
    <PageShell>
      <PageHeading
        title="Members"
        description="People with access to this workspace."
      />

      {members.isPending ? (
        <div className="mt-6 space-y-3 rounded-lg border border-border bg-card p-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="flex items-center gap-4">
              <Skeleton className="h-3.5 w-40" />
              <Skeleton className="h-3.5 w-56" />
              <Skeleton className="ml-auto h-3.5 w-16" />
            </div>
          ))}
        </div>
      ) : !data?.configured ? (
        <NotConnected />
      ) : rows.length === 0 ? (
        <p className="mt-6 text-[13px] text-muted-foreground">
          No members found.
        </p>
      ) : (
        <div className="mt-6 overflow-hidden rounded-lg border border-border bg-card">
          <table className="w-full text-[13px]">
            <thead>
              <tr className="border-b border-border text-left text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
                <th className="px-4 py-2 font-medium">
                  <span className="flex items-center gap-1.5">
                    <RiUser3Line className="size-3.5 shrink-0" />
                    Name
                  </span>
                </th>
                <th className="px-3 py-2 font-medium">
                  <span className="flex items-center gap-1.5">
                    <RiMailLine className="size-3.5 shrink-0" />
                    Email
                  </span>
                </th>
                <th className="px-4 py-2 font-medium">
                  <span className="flex items-center gap-1.5">
                    <RiShieldUserLine className="size-3.5 shrink-0" />
                    Roles
                  </span>
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {rows.map((m) => (
                <tr key={m.id} className="hover:bg-accent/40">
                  <td className="px-4 py-2 font-medium text-foreground">
                    {m.name || m.email || m.id}
                  </td>
                  <td className="px-3 py-2 text-muted-foreground">
                    {m.email || "—"}
                  </td>
                  <td className="px-4 py-2 text-muted-foreground">
                    {roleLabels(m)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </PageShell>
  )
}

function NotConnected() {
  return (
    <div className="mt-6 flex flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-border bg-card px-6 py-16 text-center">
      <p className="text-[14px] font-medium text-foreground">
        Member directory not connected
      </p>
      <p className="max-w-sm text-[13px] text-muted-foreground">
        Connecting the identity provider surfaces who&rsquo;s in this workspace.
        An admin needs to configure the directory service account.
      </p>
    </div>
  )
}

// roleLabels renders ZITADEL role codes as readable text — "ORG_OWNER" → "Owner".
function roleLabels(m: Member): string {
  if (m.roles.length === 0) return "Member"
  return m.roles
    .map((r) =>
      r
        .replace(/^ORG_/, "")
        .toLowerCase()
        .replace(/_/g, " ")
        .replace(/^\w/, (c) => c.toUpperCase())
    )
    .join(", ")
}
