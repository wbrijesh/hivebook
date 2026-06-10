"use client"

import * as React from "react"

import { userManager } from "@/lib/auth"
import { track } from "@/lib/telemetry"

// The spine's "Sign out" links here. End the ZITADEL session; ZITADEL then
// redirects back to "/" (post_logout_redirect_uri).
export default function SignedOut() {
  React.useEffect(() => {
    track("logout")
    void userManager()
      .signoutRedirect()
      .catch(() => {
        window.location.href = "/"
      })
  }, [])

  return (
    <main className="flex min-h-svh items-center justify-center bg-background text-[13px] text-muted-foreground">
      Signing out&hellip;
    </main>
  )
}
