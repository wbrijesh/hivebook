"use client"

import { useEffect, useState } from "react"

import { userManager } from "@/lib/auth"
import { track } from "@/lib/telemetry"

// OIDC redirect target — completes the PKCE code exchange, then returns home.
export default function Callback() {
  const [msg, setMsg] = useState("Completing sign-in…")

  useEffect(() => {
    userManager()
      .signinRedirectCallback()
      .then(() => {
        track("login_success")
        window.location.href = "/"
      })
      .catch((e: unknown) => {
        track("login_error", {
          message: e instanceof Error ? e.message : String(e),
        })
        setMsg(`Sign-in failed: ${e instanceof Error ? e.message : String(e)}`)
      })
  }, [])

  return <main style={{ padding: 24, fontFamily: "sans-serif" }}>{msg}</main>
}
