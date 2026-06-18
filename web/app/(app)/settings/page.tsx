"use client"

import { type ReactNode, useState } from "react"
import Link from "next/link"
import { useQueryClient } from "@tanstack/react-query"
import { useMutation, useQuery } from "@connectrpc/connect-query"
import { toast } from "sonner"
import { RiExternalLinkLine, RiPencilLine } from "@remixicon/react"

import { PageHeading, PageShell } from "@/components/app/page-kit"
import { Button } from "@/components/ui/button"
import { Flag } from "@/components/ui/flag"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"
import { useSession } from "@/lib/session"
import { regions } from "@/lib/onboarding-options"
import type { Tenant, User } from "@/lib/gen/hivebook/tenant/v1/tenant_pb"
import {
  listFeatureFlags,
  listMembers,
  setFeatureFlag,
  updateTenant,
} from "@/lib/gen/hivebook/tenant/v1/tenant-TenantService_connectquery"

// The issuer doubles as the account console origin — ZITADEL stays the system of
// record for personal identity (ADR-0013); we link out rather than rebuild it.
const issuer = process.env.NEXT_PUBLIC_OIDC_ISSUER

// Settings is the workspace + account control panel: the workspace name and data
// region (region write-once, ADR-0014), feature flags, members, and a link out to
// the identity provider for personal account management.
export default function SettingsPage() {
  const session = useSession()

  if (session.isPending) {
    return (
      <PageShell>
        <Skeleton className="h-5 w-28" />
        <Skeleton className="mt-2 h-4 w-56" />
        <div className="mt-8 space-y-3">
          <Skeleton className="h-4 w-40" />
          <Skeleton className="h-4 w-64" />
          <Skeleton className="h-20 w-full" />
        </div>
      </PageShell>
    )
  }

  const tenant = session.data?.tenant
  const user = session.data?.user

  return (
    <PageShell>
      <PageHeading title="Settings" description="Your workspace and account." />
      {tenant && <Workspace tenant={tenant} />}
      <FeatureFlags />
      <MembersSummary />
      <Account user={user} />
    </PageShell>
  )
}

// Workspace shows the org name (editable in place) and the read-only data region.
function Workspace({ tenant }: { tenant: Tenant }) {
  const qc = useQueryClient()
  const [editing, setEditing] = useState(false)
  const [name, setName] = useState(tenant.name ?? "")

  const save = useMutation(updateTenant, {
    onSuccess: () => {
      qc.invalidateQueries()
      setEditing(false)
      toast.success("Workspace updated")
    },
    onError: (e) => toast.error(`Couldn't save: ${e.message}`),
  })

  // The name is the only editable field; size/use-cases are preserved as-is so a
  // rename never wipes the onboarding answers.
  function onSave() {
    const next = name.trim()
    if (!next) return
    save.mutate({
      name: next,
      size: tenant.size ?? "",
      useCases: tenant.useCases,
      useCaseOther: tenant.useCaseOther ?? "",
    })
  }

  const r = regions.find((x) => x.id === tenant.region)

  return (
    <Section
      title="Workspace"
      description="Your organization and where its data lives. The region is fixed at setup and can't be changed."
    >
      <dl className="space-y-2 text-[13px]">
        <Row label="Name">
          {editing ? (
            <span className="flex flex-wrap items-center gap-2">
              <Input
                value={name}
                onChange={(e) => setName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") onSave()
                  if (e.key === "Escape") {
                    setName(tenant.name ?? "")
                    setEditing(false)
                  }
                }}
                placeholder="Acme Inc."
                autoFocus
                className="h-8 w-56"
              />
              <Button
                size="sm"
                onClick={onSave}
                disabled={!name.trim() || save.isPending}
              >
                {save.isPending ? "Saving…" : "Save"}
              </Button>
              <Button
                size="sm"
                variant="ghost"
                onClick={() => {
                  setName(tenant.name ?? "")
                  setEditing(false)
                }}
              >
                Cancel
              </Button>
            </span>
          ) : (
            <span className="inline-flex items-center gap-1.5">
              {tenant.name || "—"}
              <Button
                size="icon-xs"
                variant="ghost"
                aria-label="Edit name"
                onClick={() => setEditing(true)}
              >
                <RiPencilLine />
              </Button>
            </span>
          )}
        </Row>
        <Row label="Region">
          {r ? (
            <span className="inline-flex items-center gap-2">
              <Flag code={r.code} size="m" />
              {`${r.label} · ${r.city}, ${r.country}`}
            </span>
          ) : (
            (tenant.region ?? "—")
          )}
        </Row>
        <Row label="Workspace ID">
          <code className="text-[12px] text-muted-foreground">{tenant.id}</code>
        </Row>
      </dl>
    </Section>
  )
}

// FeatureFlags lists the workspace's opt-in capabilities as toggles.
function FeatureFlags() {
  const qc = useQueryClient()
  const flags = useQuery(listFeatureFlags, {})
  const set = useMutation(setFeatureFlag, {
    onSuccess: () => qc.invalidateQueries(),
    onError: (e) => toast.error(`Couldn't update: ${e.message}`),
  })

  return (
    <Section
      title="Feature flags"
      description="Opt-in capabilities for this workspace."
    >
      {flags.isPending ? (
        <div className="space-y-3 rounded-lg border border-border bg-card p-4">
          {Array.from({ length: 2 }).map((_, i) => (
            <div key={i} className="flex items-center justify-between gap-4">
              <div className="flex-1 space-y-1.5">
                <Skeleton className="h-3.5 w-48" />
                <Skeleton className="h-3 w-72" />
              </div>
              <Skeleton className="h-5 w-9 rounded-full" />
            </div>
          ))}
        </div>
      ) : (flags.data?.flags.length ?? 0) === 0 ? (
        <p className="text-[13px] text-muted-foreground">
          No feature flags available.
        </p>
      ) : (
        <div className="divide-y divide-border overflow-hidden rounded-lg border border-border bg-card">
          {flags.data!.flags.map((f) => (
            <div
              key={f.key}
              className="flex items-start justify-between gap-4 px-4 py-3"
            >
              <div className="min-w-0">
                <p className="text-[13px] font-medium text-foreground">
                  {f.name}
                </p>
                <p className="mt-0.5 text-[12px] text-muted-foreground">
                  {f.description}
                </p>
              </div>
              <Switch
                checked={f.enabled}
                disabled={set.isPending}
                onCheckedChange={(enabled) =>
                  set.mutate({ key: f.key, enabled })
                }
                aria-label={f.name}
                className="mt-0.5"
              />
            </div>
          ))}
        </div>
      )}
    </Section>
  )
}

// Account links out to the identity provider for personal profile + security.
function Account({ user }: { user?: User }) {
  const me = issuer ? `${issuer}/ui/console/users/me` : undefined
  return (
    <Section
      title="Account"
      description="Your personal identity, managed in your account."
    >
      <dl className="space-y-2 text-[13px]">
        <Row label="Name">{user?.name || "—"}</Row>
        <Row label="Email">{user?.email || "—"}</Row>
      </dl>
      {me && (
        <div className="mt-4 flex flex-wrap gap-2">
          <Button asChild variant="outline" size="sm">
            <a href={me} target="_blank" rel="noreferrer">
              Manage profile
              <RiExternalLinkLine data-icon="inline-end" />
            </a>
          </Button>
          <Button asChild variant="outline" size="sm">
            <a href={me} target="_blank" rel="noreferrer">
              Password &amp; two-factor
              <RiExternalLinkLine data-icon="inline-end" />
            </a>
          </Button>
        </div>
      )}
    </Section>
  )
}

// MembersSummary is a count + a link to the full Members surface.
function MembersSummary() {
  const members = useQuery(listMembers, {})
  const count = members.data?.members.length ?? 0
  const configured = members.data?.configured ?? false
  return (
    <Section
      title="Members"
      description="People with access to this workspace."
    >
      {members.isPending ? (
        <p className="text-[13px] text-muted-foreground">Loading…</p>
      ) : !configured ? (
        <p className="text-[13px] text-muted-foreground">
          The member directory isn&rsquo;t connected yet.
        </p>
      ) : (
        <p className="text-[13px] text-muted-foreground">
          {count} {count === 1 ? "member" : "members"}.
        </p>
      )}
      <div className="mt-3">
        <Button asChild variant="outline" size="sm">
          <Link href="/members">Manage members</Link>
        </Button>
      </div>
    </Section>
  )
}

function Section({
  title,
  description,
  children,
}: {
  title: string
  description?: string
  children: ReactNode
}) {
  return (
    <section className="mt-8 border-t border-border pt-6 first:mt-7 first:border-t-0 first:pt-0">
      <h2 className="text-[14px] font-semibold tracking-tight text-foreground">
        {title}
      </h2>
      {description && (
        <p className="mt-0.5 text-[13px] text-muted-foreground">
          {description}
        </p>
      )}
      <div className="mt-4">{children}</div>
    </section>
  )
}

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex gap-3">
      <dt className="w-32 shrink-0 text-muted-foreground">{label}</dt>
      <dd className="text-foreground">{children}</dd>
    </div>
  )
}
