"use client"

import * as React from "react"
import Link from "next/link"
import { useRouter, useSearchParams } from "next/navigation"

import { AuthCard, AuthShell } from "@/components/auth/auth-shell"
import { Logo } from "@/components/brand/logo"
import { MOCK_DELAY_MS } from "@/lib/mock-data"

// Spread the loading stages evenly across the shared mock delay so the whole
// SSO sign-in takes the same time as every other simulated action.
const STAGE_COUNT = 3
const STEP_MS = MOCK_DELAY_MS / STAGE_COUNT

function SsoCallback() {
  const router = useRouter()
  const searchParams = useSearchParams()
  // Name the provider the user actually chose, so the loader stays coherent
  // with the button they pressed (no more "Google → verifying with Okta").
  const provider = searchParams.get("provider")
  const stages = [
    `Connecting to ${provider ?? "your identity provider"}…`,
    provider
      ? `Verifying your identity with ${provider}…`
      : "Verifying your identity…",
    "Loading your workspace…",
  ]
  const [stage, setStage] = React.useState(0)

  React.useEffect(() => {
    const interval = setInterval(() => {
      setStage((s) => {
        if (s >= STAGE_COUNT - 1) {
          clearInterval(interval)
          setTimeout(() => router.push("/book"), STEP_MS)
          return s
        }
        return s + 1
      })
    }, STEP_MS)
    return () => clearInterval(interval)
  }, [router])

  return (
    <AuthShell>
      <AuthCard className="text-center">
        <div className="flex flex-col items-center gap-5 py-6">
          <Logo variant="mark" size="lg" />

          <div className="space-y-1.5">
            <h1 className="text-[18px] font-semibold tracking-tight">
              Signing you in
            </h1>
            <p className="text-[13px] text-muted-foreground">
              This should only take a moment.
            </p>
          </div>

          <ol className="mt-3 w-full max-w-[280px] space-y-1.5 text-left text-[13px]">
            {stages.map((label, i) => (
              <li
                key={label}
                className="flex items-center gap-2.5"
                aria-current={i === stage ? "step" : undefined}
              >
                <Indicator state={i < stage ? "done" : i === stage ? "active" : "idle"} />
                <span
                  className={
                    i < stage
                      ? "text-foreground"
                      : i === stage
                        ? "text-foreground"
                        : "text-muted-foreground/60"
                  }
                >
                  {label}
                </span>
              </li>
            ))}
          </ol>
        </div>
      </AuthCard>

      <div className="mt-6 text-center text-[13px] text-muted-foreground">
        Stuck on this screen?{" "}
        <Link
          href="/sign-in"
          className="font-medium text-foreground underline-offset-2 hover:underline"
        >
          Cancel and try another way
        </Link>
        .
      </div>
    </AuthShell>
  )
}

export default function SsoCallbackPage() {
  return (
    <React.Suspense>
      <SsoCallback />
    </React.Suspense>
  )
}

function Indicator({ state }: { state: "done" | "active" | "idle" }) {
  if (state === "done") {
    return (
      <span className="inline-flex size-4 items-center justify-center rounded-full bg-success text-white">
        <svg viewBox="0 0 24 24" className="size-2.5" aria-hidden>
          <path
            fill="none"
            stroke="currentColor"
            strokeWidth="3"
            strokeLinecap="round"
            strokeLinejoin="round"
            d="m5 12 5 5L20 7"
          />
        </svg>
      </span>
    )
  }
  if (state === "active") {
    return (
      <span className="size-4 animate-spin rounded-full border-2 border-muted-foreground/30 border-t-foreground" />
    )
  }
  return <span className="size-4 rounded-full border-2 border-border" />
}
