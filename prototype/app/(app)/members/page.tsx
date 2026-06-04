"use client"

import * as React from "react"
import { RiDownload2Line, RiUserAddLine } from "@remixicon/react"
import { toast } from "sonner"

import { ChromeActions } from "@/components/app/chrome"
import {
  Avatar,
  HeadRow,
  PageHeading,
  PageShell,
  Row,
  Section,
  StatBar,
  Tag,
  TableCard,
  Td,
  Th,
} from "@/components/app/page-kit"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { mockOrg } from "@/lib/mock-data"
import { MEMBERS, ROLE_LABEL, type MemberStatus, type Role } from "@/lib/mock-admin"

const ROLE_TONE: Record<Role, "info" | "muted"> = {
  owner: "info",
  admin: "info",
  member: "muted",
  viewer: "muted",
}

const STATUS: Record<MemberStatus, { tone: "success" | "warning" | "destructive"; label: string }> = {
  active: { tone: "success", label: "Active" },
  invited: { tone: "warning", label: "Invited" },
  suspended: { tone: "destructive", label: "Suspended" },
}

export default function MembersPage() {
  const [inviteOpen, setInviteOpen] = React.useState(false)
  const [email, setEmail] = React.useState("")
  const [role, setRole] = React.useState("member")

  const active = MEMBERS.filter((m) => m.status === "active").length
  const admins = MEMBERS.filter((m) => m.role === "owner" || m.role === "admin").length
  const invited = MEMBERS.filter((m) => m.status === "invited").length
  const sso = MEMBERS.filter((m) => m.provisioned === "sso").length

  function sendInvite() {
    if (!email.trim()) return
    toast.success("Invitation sent", { description: `${email} · ${ROLE_LABEL[role as Role]}` })
    setInviteOpen(false)
    setEmail("")
    setRole("member")
  }

  return (
    <>
      <ChromeActions>
        <Button size="sm" variant="outline" onClick={() => toast("Preparing export", { description: "We'll email a link when it's ready." })}>
          <RiDownload2Line className="size-3.5" />
          Export
        </Button>
        <Button size="sm" onClick={() => setInviteOpen(true)}>
          <RiUserAddLine className="size-3.5" />
          Invite member
        </Button>
      </ChromeActions>

      <Dialog open={inviteOpen} onOpenChange={setInviteOpen}>
        <DialogContent className="sm:max-w-[420px]">
          <DialogHeader>
            <DialogTitle>Invite member</DialogTitle>
            <DialogDescription>
              They'll get an email to join {mockOrg.name}. Role can be changed later.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3 py-1">
            <div className="space-y-1.5">
              <Label htmlFor="invite-email">Email</Label>
              <Input
                id="invite-email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && sendInvite()}
                placeholder="name@trenches-test.com"
                autoFocus
              />
            </div>
            <div className="space-y-1.5">
              <Label>Role</Label>
              <Select value={role} onValueChange={setRole}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="admin">Admin</SelectItem>
                  <SelectItem value="member">Member</SelectItem>
                  <SelectItem value="viewer">Viewer</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">Cancel</Button>
            </DialogClose>
            <Button onClick={sendInvite} disabled={!email.trim()}>
              Send invite
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <PageShell>
        <PageHeading
          title="Members"
          description="Who can reach the workspace, the role they hold, and how they're provisioned."
        />

        <StatBar
          items={[
            { label: "Members", value: `${MEMBERS.length}`, sub: `${active} active` },
            { label: "Admins", value: `${admins}`, sub: "owner + admin" },
            { label: "Via SSO", value: `${sso}`, sub: "Okta SCIM" },
            { label: "Pending", value: `${invited}`, sub: "invitations" },
          ]}
        />

        <Section title="People" count={MEMBERS.length}>
          <TableCard>
            <HeadRow>
              <Th className="min-w-0 flex-1">Member</Th>
              <Th className="w-24">Role</Th>
              <Th className="w-28">Provisioning</Th>
              <Th className="w-24">Status</Th>
              <Th className="w-28 text-right">Last active</Th>
            </HeadRow>
            {MEMBERS.map((m) => {
              const s = STATUS[m.status]
              return (
                <Row key={m.id} href="#">
                  <Td className="min-w-0 flex-1 gap-2.5">
                    <Avatar initial={m.initial} />
                    <span className="min-w-0">
                      <span className="block truncate text-[13px] font-medium text-foreground">
                        {m.name}
                      </span>
                      <span className="block truncate text-[12px] text-muted-foreground">
                        {m.email}
                      </span>
                    </span>
                  </Td>
                  <Td className="w-24">
                    <Tag tone={ROLE_TONE[m.role]}>{ROLE_LABEL[m.role]}</Tag>
                  </Td>
                  <Td className="w-28 text-[12px] text-muted-foreground">
                    {m.provisioned === "sso" ? "SSO" : "Manual"}
                  </Td>
                  <Td className="w-24">
                    <Tag tone={s.tone}>{s.label}</Tag>
                  </Td>
                  <Td className="w-28 justify-end text-[12px] text-muted-foreground">
                    {m.lastActive}
                  </Td>
                </Row>
              )
            })}
          </TableCard>
        </Section>
      </PageShell>
    </>
  )
}
