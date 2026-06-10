"use client"

import * as React from "react"
import { useRouter } from "next/navigation"

import { userManager } from "@/lib/auth"
import { track } from "@/lib/telemetry"
import { Logo } from "@/components/brand/logo"
import { Button } from "@/components/ui/button"

// Public entry. Signed-in users go straight to the app; everyone else gets a
// single sign-in action that hands off to ZITADEL (Authorization Code + PKCE).
export default function Home() {
  const router = useRouter()
  const [checked, setChecked] = React.useState(false)

  React.useEffect(() => {
    track("page_view", { path: "/" })
    userManager()
      .getUser()
      .then((u) => {
        if (u && !u.expired) router.replace("/book")
        else setChecked(true)
      })
      .catch(() => setChecked(true))
  }, [router])

  if (!checked) return null

  function signIn() {
    track("login_start")
    void userManager().signinRedirect()
  }

  return (
    <main className="flex min-h-svh items-center justify-center bg-background px-4">
      <div className="w-full max-w-sm text-center">
        <div className="mb-6 flex justify-center">
          <Logo size="lg" />
        </div>
        <h1 className="text-[18px] font-semibold tracking-tight text-foreground">
          Sign in to Hivebook
        </h1>
        <p className="mt-1 text-[13px] text-muted-foreground">
          Your company&rsquo;s brain.
        </p>
        <Button className="mt-6 w-full" onClick={signIn}>
          Continue with ZITADEL
        </Button>
      </div>
    </main>
  )
}
