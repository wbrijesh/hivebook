"use client"

import { useEffect, useState, type ReactNode } from "react"
import { usePathname, useRouter } from "next/navigation"
import { AnimatePresence, motion, useReducedMotion } from "motion/react"

import { OnboardingTopBar } from "@/components/onboarding/onboarding-topbar"
import { OnboardingFlowProvider } from "@/components/onboarding/onboarding-flow"
import { progress } from "@/lib/onboarding-steps"
import { userManager } from "@/lib/auth"
import { fetchTenant } from "@/lib/tenant"

// A reasonable default workspace name from the user's email domain, so step 1
// is pre-filled rather than blank.
function orgNameFromEmail(email: string): string {
  const domain = email.split("@")[1] ?? ""
  const base = domain.split(".")[0] ?? ""
  return base ? base[0].toUpperCase() + base.slice(1) : ""
}

type Initial = {
  orgName?: string
  orgSize?: string | null
  useCases?: string[]
  useCaseOther?: string
  region?: string
}

export default function OnboardingLayout({
  children,
}: {
  children: ReactNode
}) {
  const pathname = usePathname()
  const router = useRouter()
  const reduce = useReducedMotion()
  const pct = progress(pathname)

  // Gate + hydrate: must be signed in; already-onboarded tenants go to the app.
  // The flow is seeded from any partial answers already on the tenant, plus a
  // default org name from the signed-in user's email.
  const [initial, setInitial] = useState<Initial | null>(null)

  useEffect(() => {
    let alive = true
    ;(async () => {
      const u = await userManager().getUser()
      if (!u || u.expired) {
        router.replace("/")
        return
      }
      let tenant = null
      try {
        tenant = await fetchTenant()
      } catch {
        // fall through — allow onboarding to proceed even if the read failed
      }
      if (!alive) return
      if (tenant?.onboarded) {
        router.replace("/book")
        return
      }
      const email = (u.profile.email as string | undefined) ?? ""
      setInitial({
        orgName: tenant?.name || orgNameFromEmail(email),
        orgSize: tenant?.size ?? null,
        useCases: tenant?.useCases ?? [],
        useCaseOther: tenant?.useCaseOther ?? undefined,
        region: tenant?.region ?? undefined,
      })
    })()
    return () => {
      alive = false
    }
  }, [router])

  if (!initial) return null

  return (
    <OnboardingFlowProvider initial={initial}>
      <div className="flex h-svh flex-col overflow-hidden bg-background">
        <OnboardingTopBar />

        {/* Quiet fill line — forward motion, no counter. */}
        <div className="h-1 w-full shrink-0 bg-border">
          <div
            className="h-full bg-brand transition-[width] duration-500 ease-out motion-reduce:transition-none"
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
              transition={{
                duration: reduce ? 0 : 0.18,
                ease: [0.4, 0, 0.2, 1],
              }}
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
              {children}
            </motion.div>
          </AnimatePresence>
        </main>
      </div>
    </OnboardingFlowProvider>
  )
}
