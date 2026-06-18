"use client"

import { useEffect, useState } from "react"
import { Code, ConnectError, createClient } from "@connectrpc/connect"

import { transport } from "@/lib/api/client"
import {
  IntegrationService,
  type Project,
} from "@/lib/gen/hivebook/integration/v1/integration_pb"

// useProjectsStream subscribes to per-project status for one connection
// (WatchProjects) so the connector view shows live sync progress per project —
// pushed, not polled. Returns null until the first snapshot. Pass null to stay
// idle (e.g. a collapsed panel). Reconnects with a capped exponential backoff if it
// drops, and stops on a non-transient auth/permission failure (which retrying can't
// fix) — the connection-level error is surfaced by useConnectionsStream.
export function useProjectsStream(
  connectionId: string | null
): Project[] | null {
  const [projects, setProjects] = useState<Project[] | null>(null)

  useEffect(() => {
    // No reset-to-null here: the panel mounts fresh per connection, so the hook's
    // connectionId never changes under it (a sync setState in an effect would also
    // trip the react-compiler lint). Idle when there's no connection.
    if (!connectionId) return
    const ac = new AbortController()
    const client = createClient(IntegrationService, transport)
    let attempt = 0

    async function run() {
      while (!ac.signal.aborted) {
        try {
          for await (const msg of client.watchProjects(
            { connectionId: connectionId! },
            { signal: ac.signal }
          )) {
            attempt = 0 // a healthy stream resets the backoff
            setProjects(msg.projects)
          }
        } catch (e) {
          if (ac.signal.aborted) break
          // A revoked/forbidden credential won't fix itself by reconnecting.
          if (
            e instanceof ConnectError &&
            (e.code === Code.Unauthenticated ||
              e.code === Code.PermissionDenied)
          ) {
            break
          }
        }
        if (ac.signal.aborted) break
        const delay = Math.min(30_000, 1_000 * 2 ** attempt)
        attempt++
        await new Promise((resolve) => setTimeout(resolve, delay))
      }
    }

    void run()
    return () => ac.abort()
  }, [connectionId])

  return projects
}
