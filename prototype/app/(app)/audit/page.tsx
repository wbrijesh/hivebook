"use client"

import * as React from "react"
import { RiDownload2Line } from "@remixicon/react"
import { toast } from "sonner"

import { ChromeActions } from "@/components/app/chrome"
import {
  HeadRow,
  PageHeading,
  PageShell,
  Row,
  StatBar,
  Tag,
  TableCard,
  Td,
  Th,
} from "@/components/app/page-kit"
import { Button } from "@/components/ui/button"
import { AUDIT_EVENTS, type AuditKind } from "@/lib/mock-admin"
import { cn } from "@/lib/utils"

const KIND: Record<AuditKind, { tone: "muted" | "info" | "warning" | "success"; label: string }> = {
  read: { tone: "muted", label: "Read" },
  write: { tone: "info", label: "Write" },
  admin: { tone: "warning", label: "Admin" },
  auth: { tone: "success", label: "Auth" },
}

const FILTERS: (AuditKind | "all")[] = ["all", "read", "write", "admin", "auth"]

export default function AuditPage() {
  const [filter, setFilter] = React.useState<AuditKind | "all">("all")
  const events = filter === "all" ? AUDIT_EVENTS : AUDIT_EVENTS.filter((e) => e.kind === filter)

  const count = (k: AuditKind) => AUDIT_EVENTS.filter((e) => e.kind === k).length

  return (
    <>
      <ChromeActions>
        <Button
          size="sm"
          variant="outline"
          onClick={() => toast("Preparing export", { description: "Filtered audit events · CSV. We'll email a link." })}
        >
          <RiDownload2Line className="size-3.5" />
          Export
        </Button>
      </ChromeActions>

      <PageShell>
        <PageHeading
          title="Audit log"
          description="Every read and privileged write in this workspace, traceable to the change that caused it."
        />

        <StatBar
          items={[
            { label: "Events today", value: `${AUDIT_EVENTS.length}`, sub: "all actors" },
            { label: "Reads", value: `${count("read")}`, sub: "entry + ask" },
            { label: "Writes", value: `${count("write")}`, sub: "rebuilds + resolves" },
            { label: "Admin", value: `${count("admin")}`, sub: "config changes" },
          ]}
        />

        <div className="mt-6 mb-2 flex items-center gap-1">
          {FILTERS.map((f) => (
            <button
              key={f}
              type="button"
              onClick={() => setFilter(f)}
              className={cn(
                "rounded-md px-2 py-1 text-[12px] font-medium capitalize transition-colors",
                filter === f
                  ? "bg-muted text-foreground"
                  : "text-muted-foreground hover:bg-muted/50 hover:text-foreground"
              )}
            >
              {f === "all" ? "All" : KIND[f].label}
            </button>
          ))}
        </div>

        <TableCard>
          <HeadRow>
            <Th className="w-20">Time</Th>
            <Th className="w-44">Actor</Th>
            <Th className="min-w-0 flex-1">Action</Th>
            <Th className="w-20">Kind</Th>
            <Th className="w-28 text-right">Correlation</Th>
          </HeadRow>
          {events.map((e) => (
            <Row key={e.id}>
              <Td className="w-20 font-mono text-[12px] tabular-nums text-muted-foreground">
                {e.time}
              </Td>
              <Td className="w-44 truncate text-[13px] text-foreground">{e.actor}</Td>
              <Td className="min-w-0 flex-1 gap-1.5 truncate text-[13px]">
                <span className="text-muted-foreground">{e.action}</span>
                <span className="truncate text-foreground">{e.target}</span>
              </Td>
              <Td className="w-20">
                <Tag tone={KIND[e.kind].tone}>{KIND[e.kind].label}</Tag>
              </Td>
              <Td className="w-28 justify-end font-mono text-[11px] text-muted-foreground">
                {e.correlationId}
              </Td>
            </Row>
          ))}
        </TableCard>
      </PageShell>
    </>
  )
}
