"use client"

import Link from "next/link"
import { RiArrowRightSLine } from "@remixicon/react"

import { OnboardingShell } from "@/components/onboarding/onboarding-shell"
import { Button } from "@/components/ui/button"
import { useOnboardingFlow } from "@/components/onboarding/onboarding-flow"

const NEXT = [
  {
    title: "Connect your first source",
    detail: "Slack, Notion, GitHub… ingestion starts the moment you do.",
    href: "/sources",
  },
  {
    title: "Set chapter access",
    detail: "Decide who can read what.",
    href: "#",
  },
  {
    title: "Invite your team",
    detail: "Bring the people who'll use it.",
    href: "#",
  },
]

export default function CompletePage() {
  const { orgName, useCases } = useOnboardingFlow()

  return (
    <OnboardingShell
      primaryAction={
        <Button size="xl" className="px-6" asChild>
          <Link href="/book">Open your workspace</Link>
        </Button>
      }
    >
      <div className="space-y-6">
        <p className="text-[14px] leading-relaxed text-muted-foreground">
          {orgName || "Your workspace"} is ready
          {useCases.length
            ? `, with ${useCases.length} area${useCases.length > 1 ? "s" : ""} to organize`
            : ""}
          . Connect a source to start filling it in.
        </p>

        <div className="divide-y divide-border overflow-hidden rounded-xl border border-border bg-card">
          {NEXT.map((n) => (
            <Link
              key={n.title}
              href={n.href}
              className="flex items-center gap-3 px-4 py-3 transition-colors hover:bg-muted"
            >
              <div className="min-w-0 flex-1">
                <p className="text-[14px] font-medium text-foreground">
                  {n.title}
                </p>
                <p className="text-[12px] text-muted-foreground">{n.detail}</p>
              </div>
              <RiArrowRightSLine className="size-4 shrink-0 text-muted-foreground" />
            </Link>
          ))}
        </div>
      </div>
    </OnboardingShell>
  )
}
