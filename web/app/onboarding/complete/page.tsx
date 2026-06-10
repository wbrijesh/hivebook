"use client"

import * as React from "react"
import { useRouter } from "next/navigation"

import { OnboardingShell } from "@/components/onboarding/onboarding-shell"
import { Button } from "@/components/ui/button"
import { useOnboardingFlow } from "@/components/onboarding/onboarding-flow"
import { submitOnboarding } from "@/lib/tenant"
import { track } from "@/lib/telemetry"

// What comes after setup. None of these are built yet, so they're shown as a
// preview of what's next — present but disabled, never a dead link.
const NEXT = [
  {
    title: "Connect your first source",
    detail: "Slack, Notion, GitHub… ingestion starts the moment you do.",
  },
  {
    title: "Set chapter access",
    detail: "Decide who can read what.",
  },
  {
    title: "Invite your team",
    detail: "Bring the people who'll use it.",
  },
]

export default function CompletePage() {
  const router = useRouter()
  const { orgName, orgSize, useCases, useCaseOther, region } =
    useOnboardingFlow()
  const [submitting, setSubmitting] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)

  // Persist the answers, mark the tenant onboarded, then enter the app. The gate
  // (AppGate) sees the onboarded tenant and lets the shell render.
  async function finish() {
    setSubmitting(true)
    setError(null)
    try {
      await submitOnboarding({
        name: orgName,
        size: orgSize ?? "",
        region,
        useCases,
        // Only meaningful when "other" is chosen; otherwise it's cleared.
        useCaseOther: useCases.includes("other") ? useCaseOther.trim() : "",
      })
      track("onboarding_complete", { areas: useCases.length, region })
      router.push("/book")
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
      setSubmitting(false)
    }
  }

  return (
    <OnboardingShell
      primaryAction={
        <Button
          size="xl"
          className="px-6"
          onClick={finish}
          disabled={submitting}
        >
          {submitting ? "Setting up…" : "Open your workspace"}
        </Button>
      }
    >
      <div className="space-y-6">
        <p className="text-[14px] leading-relaxed text-muted-foreground">
          {orgName || "Your workspace"} is ready
          {useCases.length
            ? `, with ${useCases.length} area${useCases.length > 1 ? "s" : ""} to organize`
            : ""}
          . Here&rsquo;s what&rsquo;s coming next.
        </p>

        {error && (
          <p role="alert" className="text-[13px] text-destructive">
            {error}
          </p>
        )}

        <div className="divide-y divide-border overflow-hidden rounded-xl border border-border bg-card">
          {NEXT.map((n) => (
            <div
              key={n.title}
              aria-disabled
              className="flex items-center gap-3 px-4 py-3 opacity-55"
            >
              <div className="min-w-0 flex-1">
                <p className="text-[14px] font-medium text-foreground">
                  {n.title}
                </p>
                <p className="text-[12px] text-muted-foreground">{n.detail}</p>
              </div>
              <span className="shrink-0 rounded-full border border-border px-2 py-0.5 text-[10px] font-medium tracking-wide text-muted-foreground uppercase">
                Soon
              </span>
            </div>
          ))}
        </div>
      </div>
    </OnboardingShell>
  )
}
