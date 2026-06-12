"use client"

import { useEffect, useRef, useState } from "react"

import { userManager } from "@/lib/auth"
import { track } from "@/lib/telemetry"
import { Logo } from "@/components/brand/logo"

// OIDC redirect target — completes the PKCE code exchange, then returns home.
export default function Callback() {
  const [error, setError] = useState<string | null>(null)
  const ran = useRef(false)

  useEffect(() => {
    // Strict Mode invokes effects twice in dev; the auth code is single-use, so a
    // second exchange fails. Guard it to avoid the spurious "sign-in failed" flash.
    if (ran.current) return
    ran.current = true

    userManager()
      .signinRedirectCallback()
      .then(() => {
        track("login_success")
        window.location.href = "/"
      })
      .catch((e: unknown) => {
        const msg = e instanceof Error ? e.message : String(e)
        track("login_error", { message: msg })
        setError(msg)
      })
  }, [])

  return (
    <main className="flex min-h-svh flex-col items-center justify-center gap-4 bg-background px-4 text-center">
      <Logo size="lg" />
      {error ? (
        <p role="alert" className="text-[13px] text-destructive">
          Sign-in failed: {error}
        </p>
      ) : (
        <p className="text-[13px] text-muted-foreground">Completing sign-in…</p>
      )}
    </main>
  )
}
