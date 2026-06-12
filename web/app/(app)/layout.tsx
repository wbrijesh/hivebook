import type { ReactNode } from "react"

import { AppSpine, SpineProvider } from "@/components/app/spine"
import { AppContextBar } from "@/components/app/app-context-bar"
import { AppContextPanel } from "@/components/app/app-context-panel"
import { AppGate } from "@/components/app/app-gate"

// The signed-in application shell — a system frame, not a website chrome.
// A dark icon-rail spine (global surfaces) · a section-contextual panel ·
// a dense context bar (breadcrumb + page actions) · the work area. Dense,
// keyboard-first, instant navigation. Gated by AppGate (OIDC + onboarded).
export default function AppLayout({ children }: { children: ReactNode }) {
  return (
    <AppGate>
      <SpineProvider>
        <div className="flex h-svh overflow-hidden bg-background">
          <AppSpine />
          <div className="flex min-w-0 flex-1 flex-col">
            <AppContextBar />
            <div className="flex min-h-0 flex-1">
              <AppContextPanel />
              <main className="min-w-0 flex-1 overflow-hidden">{children}</main>
            </div>
          </div>
        </div>
      </SpineProvider>
    </AppGate>
  )
}
