"use client"

import { API_BASE } from "@/lib/auth"

// The closed set of browser telemetry events. A union (not a free string) so a
// typo is a compile error and the event vocabulary stays stable and greppable —
// the frontend mirror of the backend's stable event-name rule (logging.adoc).
export type TelemetryEvent =
  | "page_view"
  | "login_start"
  | "login_success"
  | "login_error"
  | "logout"
  | "onboarding_complete"

// Fire-and-forget browser telemetry. Posts to the Go API's /events endpoint,
// which records each event as a structured log (VictoriaLogs) and a metric
// (hivebook_web_events_total). Never throws — telemetry must not break the UI.
export function track(
  type: TelemetryEvent,
  props?: Record<string, unknown>
): void {
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
