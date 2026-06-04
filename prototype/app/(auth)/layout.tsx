"use client"

import { useContext, useRef, type ReactNode } from "react"
import { usePathname } from "next/navigation"
import { AnimatePresence, motion, useReducedMotion } from "motion/react"
import { LayoutRouterContext } from "next/dist/shared/lib/app-router-context.shared-runtime"

import { Logo } from "@/components/brand/logo"
import { SiteFooter } from "@/components/brand/site-footer"
import { AuthBgPattern } from "@/components/brand/auth-bg-pattern"
import { StylePicker } from "@/components/brand/style-picker"
import { AuthFlowProvider } from "@/components/auth/auth-flow"

// Freezes the router context for the exiting subtree, so the outgoing page
// keeps rendering its own content during its exit animation. Without this, App
// Router swaps in the new route immediately and the exit would animate the new
// page's content instead of the old one's.
function FrozenRouter({ children }: { children: ReactNode }) {
  const context = useContext(LayoutRouterContext)
  const frozen = useRef(context).current
  return (
    <LayoutRouterContext.Provider value={frozen}>
      {children}
    </LayoutRouterContext.Provider>
  )
}

export default function AuthLayout({ children }: { children: ReactNode }) {
  const pathname = usePathname()
  const reduce = useReducedMotion()

  return (
    <AuthFlowProvider>
      <div className="relative isolate flex min-h-svh flex-col bg-background">
        <AuthBgPattern />

        {/* Chrome stays fixed across transitions. */}
        <header className="relative z-10 flex shrink-0 items-center justify-between gap-3 border-b border-border bg-background px-6 py-3">
          <Logo />
          {/* DEMO ONLY — the theming picker and the marketing link are dev/
              showcase affordances; strip from the real auth chrome before ship. */}
          <div className="flex items-center gap-3">
            <a
              href="#"
              className="text-[13px] text-muted-foreground transition-colors hover:text-foreground"
            >
              Enterprise or on-prem?{" "}
              <span className="font-medium text-foreground">Talk to us</span>
            </a>
            <StylePicker />
          </div>
        </header>

        <main className="relative z-10 flex flex-1 items-center justify-center px-4 py-8">
          {/* A quick cross-fade with a few px of settle — reads as "next step
              in the same task," not "a journey to another room." Short-circuits
              under reduced-motion. mode="wait" + FrozenRouter keeps the outgoing
              page rendering its own content while it leaves. */}
          <AnimatePresence mode="wait" initial={false}>
            <motion.div
              key={pathname}
              initial={{ opacity: 0, y: reduce ? 0 : 4 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: reduce ? 0 : -4 }}
              transition={{ duration: reduce ? 0 : 0.18, ease: [0.4, 0, 0.2, 1] }}
              // Focus the page's primary field only once the enter settles
              // (opacity === 1) — focusing mid-transition causes scroll jumps,
              // and skipping it on first load avoids yanking SR users past the
              // heading.
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
              className="w-full"
            >
              <FrozenRouter>{children}</FrozenRouter>
            </motion.div>
          </AnimatePresence>
        </main>

        <SiteFooter />
      </div>
    </AuthFlowProvider>
  )
}
