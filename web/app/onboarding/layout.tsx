"use client"

import { useEffect, type ReactNode } from "react"
import { usePathname, useRouter } from "next/navigation"
import { AnimatePresence, motion, useReducedMotion } from "motion/react"

import { OnboardingTopBar } from "@/components/onboarding/onboarding-topbar"
import { OnboardingFlowProvider } from "@/components/onboarding/onboarding-flow"
import { progress } from "@/lib/onboarding-steps"
import { useSession, isAuthError } from "@/lib/session"

// A reasonable default workspace name from the user's email domain, so step 1
// is pre-filled rather than blank.
function orgNameFromEmail(email: string): string {
  const domain = email.split("@")[1] ?? ""
  const base = domain.split(".")[0] ?? ""
  return base ? base[0].toUpperCase() + base.slice(1) : ""
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

  // Gate + hydrate from the server session (design-doc 0005/0006): must be signed
  // in; already-onboarded tenants go to the app. The flow is seeded from any
  // partial answers already on the tenant, plus a default org name from the
  // signed-in user's email.
  const { data, isPending, error } = useSession()
  const onboarded = data?.tenant?.onboarded === true

  useEffect(() => {
    if (isPending) return
    if (error) {
      if (isAuthError(error)) router.replace("/")
      return
    }
    if (onboarded) router.replace("/book")
  }, [isPending, error, onboarded, router])

  // Loading, or redirecting (signed out / already onboarded) — render nothing.
  if (isPending || error || !data?.tenant || !data.user || onboarded)
    return null

  const t = data.tenant
  // Coalesce every field to a concrete default: a present key with an `undefined`
  // value would override the provider's defaults through the spread, leaving
  // non-optional fields (region, useCaseOther) actually undefined at runtime.
  const initial = {
    orgName: t.name || orgNameFromEmail(data.user.email),
    orgSize: t.size ?? null,
    useCases: t.useCases ?? [],
    useCaseOther: t.useCaseOther ?? "",
    region: t.region ?? "",
  }

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
