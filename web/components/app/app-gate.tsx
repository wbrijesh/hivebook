"use client"

import * as React from "react"
import { useRouter } from "next/navigation"

import { userManager } from "@/lib/auth"
import { fetchTenant } from "@/lib/tenant"

// Gates the application shell: render it only for a signed-in AND onboarded
// tenant. Signed-out → "/"; signed-in but not onboarded → "/onboarding"
// (design-doc 0005).
//
// RBAC: gates on authentication + onboarding only, not role — v0.1 treats every
// signed-in user as an admin. Add role checks when the role model lands.
export function AppGate({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const [ready, setReady] = React.useState(false)

  React.useEffect(() => {
    let alive = true
    ;(async () => {
      const u = await userManager().getUser()
      if (!u || u.expired) {
        router.replace("/")
        return
      }
      try {
        const t = await fetchTenant()
        if (!alive) return
        if (!t || !t.onboarded) router.replace("/onboarding")
        else setReady(true)
      } catch {
        // The API rejected the token (stale/expired). Clear it so "/" shows a
        // clean sign-in instead of looping back into the gate.
        if (!alive) return
        await userManager()
          .removeUser()
          .catch(() => {})
        router.replace("/")
      }
    })()
    return () => {
      alive = false
    }
  }, [router])

  if (!ready) return null
  return <>{children}</>
}
