"use client"

import * as React from "react"
import { useRouter } from "next/navigation"

import { userManager } from "@/lib/auth"
import { useSession, isAuthError } from "@/lib/session"
import { LoadingScreen } from "@/components/brand/loading-screen"

// Gates the application shell: render it only for a signed-in AND onboarded
// tenant. Signed-out (token rejected) → "/"; signed-in but not onboarded →
// "/onboarding" (design-doc 0005).
//
// Session policy (design-doc 0006): only an auth failure ends the session. A
// transient failure (server down, network) is retried by Query and surfaced —
// it never signs the user out or loops them back to sign-in.
//
// RBAC: gates on authentication + onboarding only, not role — v0.1 treats every
// signed-in user as an admin. Add role checks when the role model lands.
export function AppGate({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const { data, isPending, error, refetch } = useSession()

  const onboarded = data?.tenant?.onboarded === true

  React.useEffect(() => {
    if (isPending) return
    if (error) {
      if (isAuthError(error)) {
        // Token rejected (stale/expired). Clear it so "/" shows a clean sign-in
        // instead of looping back into the gate.
        userManager()
          .removeUser()
          .catch(() => {})
        router.replace("/")
      }
      return
    }
    if (data && !onboarded) router.replace("/onboarding")
  }, [data, onboarded, isPending, error, router])

  // Persistent non-auth failure: don't sign out — let the user retry.
  if (error && !isAuthError(error)) {
    return (
      <div
        role="alert"
        aria-live="polite"
        className="flex min-h-svh flex-col items-center justify-center gap-3 bg-background px-4 text-center"
      >
        <p className="text-[13px] text-muted-foreground">
          Couldn&rsquo;t reach the server.
        </p>
        <button
          onClick={() => refetch()}
          className="rounded-md border border-border px-3 py-1.5 text-[13px] text-foreground transition-colors hover:bg-muted"
        >
          Try again
        </button>
      </div>
    )
  }

  // Pending, redirecting to sign-in, or redirecting to onboarding.
  if (isPending || !onboarded) return <LoadingScreen />
  return <>{children}</>
}
