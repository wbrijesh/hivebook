import { cn } from "@/lib/utils"
import {
  type Connection,
  ConnectionStatus,
} from "@/lib/gen/hivebook/integration/v1/integration_pb"

// connectionStatus maps a connection's backend status to a human line + a tone —
// the single source of truth for how a connection's health reads in the UI.
export function connectionStatus(c: Connection): {
  text: string
  tone: "ok" | "muted" | "error"
} {
  switch (c.status) {
    case ConnectionStatus.CONNECTED:
      return { text: "Connected", tone: "ok" }
    case ConnectionStatus.SYNCING:
      return { text: "Updating…", tone: "muted" }
    case ConnectionStatus.STOPPING:
      return { text: "Stopping…", tone: "muted" }
    case ConnectionStatus.NEEDS_REAUTH:
      return { text: "Disconnected — reconnect needed", tone: "error" }
    case ConnectionStatus.ERROR:
      return {
        text: c.lastError ? `Error · ${c.lastError}` : "Error",
        tone: "error",
      }
    default:
      return { text: "Connected", tone: "ok" }
  }
}

export function ConnectionStatusText({
  connection,
}: {
  connection: Connection
}) {
  const s = connectionStatus(connection)
  return (
    <span
      className={cn(
        "text-[12px]",
        s.tone === "error" ? "text-destructive" : "text-muted-foreground"
      )}
    >
      {s.text}
    </span>
  )
}
