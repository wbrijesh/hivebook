"use client"

import { useContext, useRef, type ReactNode } from "react"
import { usePathname } from "next/navigation"
import { AnimatePresence, motion, useReducedMotion } from "motion/react"
import { LayoutRouterContext } from "next/dist/shared/lib/app-router-context.shared-runtime"

import { OnboardingTopBar } from "@/components/onboarding/onboarding-topbar"
import { OnboardingFlowProvider } from "@/components/onboarding/onboarding-flow"
import { progress } from "@/lib/onboarding-steps"
import { mockUser } from "@/lib/mock-data"

// Keeps the exiting step rendering its own content during the exit animation
// (see the auth layout for the full explanation).
function FrozenRouter({ children }: { children: ReactNode }) {
  const context = useContext(LayoutRouterContext)
  const frozen = useRef(context).current
  return (
    <LayoutRouterContext.Provider value={frozen}>
      {children}
    </LayoutRouterContext.Provider>
  )
}

// A reasonable default workspace name from the user's email domain, so step 1
// is pre-filled rather than blank.
function orgNameFromEmail(email: string): string {
  const domain = email.split("@")[1] ?? ""
  const base = domain.split(".")[0] ?? ""
  return base ? base[0].toUpperCase() + base.slice(1) : ""
}

export default function OnboardingLayout({ children }: { children: ReactNode }) {
  const pathname = usePathname()
  const reduce = useReducedMotion()
  const pct = progress(pathname)

  return (
    <OnboardingFlowProvider initial={{ orgName: orgNameFromEmail(mockUser.email) }}>
      <div className="flex h-svh flex-col overflow-hidden bg-background">
        <OnboardingTopBar />

        {/* Quiet fill line — forward motion, no counter. Persistent, so it
            animates a continuous fill rather than flickering per route. */}
        <div className="h-1 w-full shrink-0 bg-border">
          <div
            className="h-full bg-brand transition-[width] duration-500 ease-out"
            style={{ width: `${pct * 100}%` }}
          />
        </div>

        <main className="relative flex-1 overflow-hidden">
          <AnimatePresence mode="wait" initial={false}>
            <motion.div
              key={pathname}
              initial={{ opacity: 0, y: reduce ? 0 : 4 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: reduce ? 0 : -4 }}
              transition={{ duration: reduce ? 0 : 0.18, ease: [0.4, 0, 0.2, 1] }}
              onAnimationComplete={(def) => {
                if (
                  def &&
                  typeof def === "object" &&
                  (def as { opacity?: number }).opacity === 1
                ) {
                  document
                    .querySelector<HTMLElement>("[data-autofocus]")
                    ?.focus()
                }
              }}
              className="h-full"
            >
              <FrozenRouter>{children}</FrozenRouter>
            </motion.div>
          </AnimatePresence>
        </main>
      </div>
    </OnboardingFlowProvider>
  )
}
