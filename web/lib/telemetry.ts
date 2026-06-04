"use client"

import { API_BASE } from "@/lib/auth"

// Fire-and-forget browser telemetry. Posts to the Go API's /events endpoint,
// which records each event as a structured log (VictoriaLogs) and a metric
// (trenches_web_events_total). Never throws — telemetry must not break the UI.
export function track(type: string, props?: Record<string, unknown>): void {
  try {
    void fetch(`${API_BASE}/events`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ type, props }),
      keepalive: true,
    }).catch(() => {})
  } catch {
    // ignore
  }
}
