"use client"

import { useEffect, useState } from "react"
import { Code, ConnectError, createClient } from "@connectrpc/connect"

import { transport } from "@/lib/api/client"
import {
  type Connection,
  IntegrationService,
} from "@/lib/gen/hivebook/integration/v1/integration_pb"

// useConnectionsStream subscribes to the server-streamed connection snapshots
// (WatchConnections) so the Sources UI reflects sync state live — no polling, no
// manual refresh. The server pushes a fresh snapshot on subscribe and again on
// every change (Postgres LISTEN/NOTIFY upstream).
//
// Returns { connections, error }. connections is null until the first snapshot.
// error is set only on a non-transient failure (auth/permission), at which point
// the stream STOPS reconnecting — retrying a revoked token just hot-loops. Transient
// drops reconnect with a capped exponential backoff and leave error null, so a blip
// self-heals (each reconnect re-sends a full snapshot).
export function useConnectionsStream(): {
  connections: Connection[] | null
  error: ConnectError | null
} {
  const [connections, setConnections] = useState<Connection[] | null>(null)
  const [error, setError] = useState<ConnectError | null>(null)

  useEffect(() => {
    const ac = new AbortController()
    const client = createClient(IntegrationService, transport)
    let attempt = 0

    async function run() {
      while (!ac.signal.aborted) {
        try {
          for await (const msg of client.watchConnections(
            {},
            { signal: ac.signal }
          )) {
            attempt = 0 // a healthy stream resets the backoff
            setConnections(msg.connections)
            setError(null)
          }
        } catch (e) {
          if (ac.signal.aborted) break
          // A revoked/forbidden credential won't fix itself by reconnecting —
          // surface it and stop (matches the Query layer's auth handling).
          if (isAuthError(e)) {
            setError(e as ConnectError)
            break
          }
        }
        if (ac.signal.aborted) break
        // Capped exponential backoff so a sustained outage doesn't hot-loop.
        const delay = Math.min(30_000, 1_000 * 2 ** attempt)
        attempt++
        await new Promise((resolve) => setTimeout(resolve, delay))
      }
    }

    void run()
    return () => ac.abort()
  }, [])

  return { connections, error }
}

// isAuthError reports a non-transient credential failure (don't reconnect).
function isAuthError(e: unknown): boolean {
  return (
    e instanceof ConnectError &&
    (e.code === Code.Unauthenticated || e.code === Code.PermissionDenied)
  )
}
