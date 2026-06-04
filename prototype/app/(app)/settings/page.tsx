"use client"

import type { ReactNode } from "react"
import { RiLock2Line } from "@remixicon/react"
import { toast } from "sonner"

import { PageHeading, PageShell, Tag } from "@/components/app/page-kit"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { mockOrg, regions } from "@/lib/mock-data"
import { cn } from "@/lib/utils"

export default function SettingsPage() {
  const region = regions.find((r) => r.id === mockOrg.region)

  return (
    <PageShell width="reading">
      <PageHeading title="Settings" description="Workspace configuration, security, and data lifecycle." />

      <Tabs defaultValue="workspace" className="mt-5">
        <TabsList>
          <TabsTrigger value="workspace">Workspace</TabsTrigger>
          <TabsTrigger value="residency">Residency</TabsTrigger>
          <TabsTrigger value="security">Security</TabsTrigger>
          <TabsTrigger value="data">Data</TabsTrigger>
        </TabsList>

        <TabsContent value="workspace">
          <Card>
            <SettingRow label="Workspace name" description="Shown across the app and in invitations.">
              <Input defaultValue={mockOrg.name} className="h-8 w-56 text-[13px]" />
            </SettingRow>
            <SettingRow label="Workspace URL" description="The slug in your workspace address.">
              <div className="flex items-center gap-1 text-[13px] text-muted-foreground">
                trenches.app/<span className="font-medium text-foreground">{mockOrg.slug}</span>
              </div>
            </SettingRow>
            <SettingRow label="Primary domain" description="Verified email domain for SSO and invites.">
              <span className="text-[13px] text-foreground">{mockOrg.domain}</span>
            </SettingRow>
            <SettingRow label="Default access" description="Chapters new members can read.">
              <Tag>All chapters</Tag>
            </SettingRow>
          </Card>
        </TabsContent>

        <TabsContent value="residency">
          <Card>
            <SettingRow label="Region" description="Where this workspace's data is processed and stored.">
              <span className="flex items-center gap-1.5 text-[13px] text-foreground">
                {region?.flag} {region?.city} · {region?.label}
              </span>
            </SettingRow>
            <SettingRow label="Data residency" description="Region is fixed after setup; changing it requires migration.">
              <Tag tone="muted">
                <RiLock2Line className="mr-1 size-3" />
                Locked
              </Tag>
            </SettingRow>
            <SettingRow label="Encryption at rest" description="AES-256 on all stored artifacts and summaries.">
              <Tag tone="success">Enabled</Tag>
            </SettingRow>
          </Card>
        </TabsContent>

        <TabsContent value="security">
          <Card>
            <SettingRow label="Single sign-on" description="SAML / OIDC via your identity provider.">
              <span className="flex items-center gap-2 text-[13px]">
                <Tag tone="success">Okta · SAML</Tag>
                <Button size="xs" variant="outline" onClick={() => toast("SSO configuration", { description: "SAML metadata, attribute mapping, and enforcement." })}>
                  Configure
                </Button>
              </span>
            </SettingRow>
            <SettingRow label="Enforce SSO" description="Require all members to sign in through SSO.">
              <Tag tone="success">On</Tag>
            </SettingRow>
            <SettingRow label="SCIM provisioning" description="Sync members and deprovisioning from your IdP.">
              <Tag tone="success">Connected</Tag>
            </SettingRow>
            <SettingRow label="Customer-managed keys" description="Bring your own KMS key (Enterprise).">
              <Tag tone="muted">
                <RiLock2Line className="mr-1 size-3" />
                Enterprise
              </Tag>
            </SettingRow>
          </Card>
        </TabsContent>

        <TabsContent value="data">
          <Card>
            <SettingRow label="Export workspace" description="A full archive of the book and raw corpus.">
              <Button size="xs" variant="outline" onClick={() => toast("Preparing export", { description: "A full archive. We'll email a link." })}>
                Request export
              </Button>
            </SettingRow>
            <SettingRow label="Suspend ingestion" description="Pause all source syncing without losing data.">
              <Button size="xs" variant="outline" onClick={() => toast("Ingestion suspended", { description: "Syncing is paused. Resume any time." })}>
                Suspend
              </Button>
            </SettingRow>
          </Card>

          <div className="mt-5 overflow-hidden rounded-md border border-destructive/40">
            <div className="border-b border-destructive/30 bg-destructive/5 px-3.5 py-2 text-[11px] font-medium uppercase tracking-wide text-destructive">
              Danger zone
            </div>
            <div className="divide-y divide-border px-3.5">
              <SettingRow label="Archive workspace" description="Make it read-only and stop all processing.">
                <Button
                  size="xs"
                  variant="outline"
                  className="border-destructive/50 text-destructive hover:bg-destructive/10"
                  onClick={() => toast.warning("Archive workspace?", { description: "This makes everything read-only." })}
                >
                  Archive
                </Button>
              </SettingRow>
              <SettingRow label="Delete workspace" description="Permanently remove the book, corpus, and history.">
                <Button
                  size="xs"
                  variant="outline"
                  className="border-destructive/50 text-destructive hover:bg-destructive/10"
                  onClick={() => toast.error("Delete workspace?", { description: "This is permanent and cannot be undone." })}
                >
                  Delete
                </Button>
              </SettingRow>
            </div>
          </div>
        </TabsContent>
      </Tabs>
    </PageShell>
  )
}

function Card({ children }: { children: ReactNode }) {
  return (
    <div className="mt-4 rounded-md border border-border bg-card px-3.5">{children}</div>
  )
}

function SettingRow({
  label,
  description,
  children,
}: {
  label: string
  description?: string
  children: ReactNode
}) {
  return (
    <div className={cn("flex items-center justify-between gap-4 border-b border-border py-3.5 last:border-b-0")}>
      <div className="min-w-0">
        <div className="text-[13px] font-medium text-foreground">{label}</div>
        {description && <div className="mt-0.5 text-[12px] text-muted-foreground">{description}</div>}
      </div>
      <div className="shrink-0">{children}</div>
    </div>
  )
}
