"use client"

import type { ReactNode } from "react"
import { toast } from "sonner"

import { PageHeading, PageShell, Tag } from "@/components/app/page-kit"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { mockUser } from "@/lib/mock-data"

const NOTIFICATIONS = [
  { id: "digest", label: "Weekly digest", description: "A summary of what changed in your chapters.", on: true },
  { id: "mentions", label: "Mentions", description: "When you're named in a review or comment.", on: true },
  { id: "rebuilds", label: "Rebuild completed", description: "When a summary you requested finishes rebuilding.", on: false },
  { id: "proposals", label: "Review proposals", description: "When the system parks a structural decision.", on: false },
]

const SESSIONS = [
  { id: "s1", device: "Safari · macOS", meta: "Washington DC · this device", time: "now", current: true },
  { id: "s2", device: "Safari · iOS", meta: "Washington DC", time: "2 days ago" },
  { id: "s3", device: "Chrome · Windows", meta: "New York", time: "1 week ago" },
]

export default function AccountPage() {
  return (
    <PageShell width="reading">
      <PageHeading title="Account" description="Your personal settings — separate from the workspace." />

      <Tabs defaultValue="profile" className="mt-5">
        <TabsList>
          <TabsTrigger value="profile">Profile</TabsTrigger>
          <TabsTrigger value="notifications">Notifications</TabsTrigger>
          <TabsTrigger value="security">Security</TabsTrigger>
        </TabsList>

        <TabsContent value="profile">
          <Card>
            <Row label="Avatar" description="Shown next to your activity.">
              <span className="flex size-9 items-center justify-center rounded-full bg-brand-subtle text-[14px] font-semibold text-brand-subtle-foreground">
                {mockUser.initial}
              </span>
            </Row>
            <Row label="Name" description="Your display name across the workspace.">
              <Input defaultValue={mockUser.name} className="h-8 w-56 text-[13px]" />
            </Row>
            <Row label="Email" description="Managed by your identity provider.">
              <span className="text-[13px] text-foreground">{mockUser.email}</span>
            </Row>
            <Row label="Role" description="Your permission level in this workspace.">
              <Tag tone="info">{mockUser.role}</Tag>
            </Row>
          </Card>
        </TabsContent>

        <TabsContent value="notifications">
          <Card>
            {NOTIFICATIONS.map((n) => (
              <Row key={n.id} label={n.label} description={n.description}>
                <Switch defaultChecked={n.on} />
              </Row>
            ))}
          </Card>
        </TabsContent>

        <TabsContent value="security">
          <Card>
            <Row label="Two-factor authentication" description="Enforced through your identity provider.">
              <Tag tone="success">Via Okta</Tag>
            </Row>
            <Row label="Password" description="Sign-in is handled by SSO; no password to manage.">
              <Tag tone="muted">SSO only</Tag>
            </Row>
          </Card>

          <div className="mt-5">
            <div className="mb-2 flex items-center justify-between">
              <h2 className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                Active sessions
              </h2>
              <Button
                size="xs"
                variant="outline"
                onClick={() => toast.success("Signed out other sessions", { description: "Only this device stays signed in." })}
              >
                Sign out all others
              </Button>
            </div>
            <div className="overflow-hidden rounded-md border border-border bg-card">
              {SESSIONS.map((s) => (
                <div
                  key={s.id}
                  className="flex items-center gap-3 border-b border-border px-3.5 py-2.5 last:border-b-0"
                >
                  <span className="min-w-0 flex-1">
                    <span className="flex items-center gap-2 text-[13px] font-medium text-foreground">
                      {s.device}
                      {s.current && <Tag tone="success">This device</Tag>}
                    </span>
                    <span className="block text-[12px] text-muted-foreground">{s.meta}</span>
                  </span>
                  <span className="text-[12px] text-muted-foreground">{s.time}</span>
                </div>
              ))}
            </div>
          </div>
        </TabsContent>
      </Tabs>
    </PageShell>
  )
}

function Card({ children }: { children: ReactNode }) {
  return <div className="mt-4 rounded-md border border-border bg-card px-3.5">{children}</div>
}

function Row({
  label,
  description,
  children,
}: {
  label: string
  description?: string
  children: ReactNode
}) {
  return (
    <div className="flex items-center justify-between gap-4 border-b border-border py-3.5 last:border-b-0">
      <div className="min-w-0">
        <div className="text-[13px] font-medium text-foreground">{label}</div>
        {description && <div className="mt-0.5 text-[12px] text-muted-foreground">{description}</div>}
      </div>
      <div className="shrink-0">{children}</div>
    </div>
  )
}
