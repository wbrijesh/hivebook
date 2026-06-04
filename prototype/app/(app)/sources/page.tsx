"use client"

import { RiAddLine, RiRefreshLine } from "@remixicon/react"
import { toast } from "sonner"

import { ChromeActions } from "@/components/app/chrome"
import {
  Dot,
  HeadRow,
  Num,
  PageHeading,
  PageShell,
  Row,
  Section,
  StatBar,
  TableCard,
  Td,
  Th,
  fmtCount,
} from "@/components/app/page-kit"
import { ConnectorIcon } from "@/components/brand/connector-icon"
import { Button } from "@/components/ui/button"
import { SOURCES, type SourceHealth } from "@/lib/mock-book"
import { connectors } from "@/lib/mock-data"

const HEALTH: Record<SourceHealth, { tone: "success" | "info" | "warning" | "destructive"; label: string; pulse?: boolean }> = {
  synced: { tone: "success", label: "Synced" },
  syncing: { tone: "info", label: "Syncing…", pulse: true },
  "auth-lapse": { tone: "warning", label: "Auth expired" },
  failed: { tone: "destructive", label: "Failed" },
}

const MODE: Record<string, string> = {
  slack: "Stream",
  github: "Stream",
  notion: "Poll · 15m",
  jira: "Poll · 15m",
  gdrive: "Poll · 1h",
}

// Sources already wired up — don't offer them again in the catalog.
const CONNECTED = new Set(["slack", "notion", "jira", "github", "drive"])

export default function SourcesPage() {
  const healthy = SOURCES.filter((s) => s.health === "synced" || s.health === "syncing").length
  const artifacts = SOURCES.reduce((n, s) => n + s.artifacts, 0)
  const needsAttention = SOURCES.filter((s) => s.health === "failed" || s.health === "auth-lapse").length

  return (
    <>
      <ChromeActions>
        <Button
          size="sm"
          variant="outline"
          onClick={() => toast("Sync started", { description: "Re-pulling from all connected sources." })}
        >
          <RiRefreshLine className="size-3.5" />
          Sync all
        </Button>
        <Button
          size="sm"
          onClick={() => toast("Choose a connector below to get started.")}
        >
          <RiAddLine className="size-3.5" />
          Connect source
        </Button>
      </ChromeActions>

      <PageShell>
        <PageHeading
          title="Sources"
          description="The systems your knowledge already lives in, and the health of their ingestion."
        />

        <StatBar
          items={[
            { label: "Connected", value: `${SOURCES.length}`, sub: `${healthy} healthy` },
            { label: "Artifacts", value: fmtCount(artifacts), sub: "ingested" },
            { label: "Needs attention", value: `${needsAttention}`, sub: needsAttention ? "review below" : "all clear" },
            { label: "Last sync", value: "2 min", sub: "ago" },
          ]}
        />

        <Section title="Connected" count={SOURCES.length}>
          <TableCard>
            <HeadRow>
              <Th className="min-w-0 flex-1">Source</Th>
              <Th className="w-24">Mode</Th>
              <Th className="w-20 text-right">Artifacts</Th>
              <Th className="w-20 text-right">Channels</Th>
              <Th className="w-28">Health</Th>
              <Th className="w-24 text-right">Last sync</Th>
            </HeadRow>
            {SOURCES.map((s) => {
              const h = HEALTH[s.health]
              return (
                <Row key={s.id} href="#">
                  <Td className="min-w-0 flex-1 gap-2.5">
                    <ConnectorIcon color={s.color} letter={s.letter} size="sm" className="size-6" />
                    <span className="truncate font-medium text-foreground">{s.name}</span>
                  </Td>
                  <Td className="w-24 text-[12px] text-muted-foreground">{MODE[s.kind] ?? "Poll"}</Td>
                  <Num className="w-20">{s.artifacts ? fmtCount(s.artifacts) : "—"}</Num>
                  <Num className="w-20">{s.containers || "—"}</Num>
                  <Td className="w-28 gap-1.5">
                    <Dot tone={h.tone} pulse={h.pulse} />
                    <span className="text-[12px] text-foreground">{h.label}</span>
                  </Td>
                  <Td className="w-24 justify-end text-[12px] text-muted-foreground">{s.lastSync}</Td>
                </Row>
              )
            })}
          </TableCard>
        </Section>

        <Section title="Available connectors" count={connectors.length}>
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4">
            {connectors
              .filter((c) => !CONNECTED.has(c.id))
              .map((c) => (
                <button
                  key={c.id}
                  type="button"
                  onClick={() => toast(`Connecting ${c.name}…`, { description: "You'll be sent to authorize access." })}
                  className="group flex items-center gap-2.5 rounded-md border border-border bg-card px-3 py-2.5 text-left transition-colors hover:bg-muted/50"
                >
                  <ConnectorIcon color={c.color} letter={c.letter} size="sm" className="size-7" />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-[13px] font-medium text-foreground">
                      {c.name}
                    </span>
                    <span className="block truncate text-[12px] text-muted-foreground">
                      {c.category}
                    </span>
                  </span>
                  <RiAddLine className="size-4 shrink-0 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100" />
                </button>
              ))}
          </div>
        </Section>
      </PageShell>
    </>
  )
}
