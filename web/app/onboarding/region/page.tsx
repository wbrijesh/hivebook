"use client"

import { useRouter, usePathname } from "next/navigation"

import {
  OnboardingShell,
  OnboardingBack,
  OnboardingContinue,
} from "@/components/onboarding/onboarding-shell"
import {
  OptionGrid,
  type Option,
} from "@/components/onboarding/onboarding-form"
import { useOnboardingFlow } from "@/components/onboarding/onboarding-flow"
import { regions } from "@/lib/onboarding-options"
import { nextPath, prevPath } from "@/lib/onboarding-steps"

// Recommended first; no flags (no emoji). Region code + city in the detail
// line distinguishes same-country clusters.
const REGION_OPTIONS: Option[] = [...regions]
  .sort((a, b) => Number(!!b.recommended) - Number(!!a.recommended))
  .map((r) => ({
    id: r.id,
    label: r.country,
    detail: `${r.city} · ${r.label}${r.recommended ? " · Recommended" : ""}`,
  }))

export default function RegionPage() {
  const router = useRouter()
  const pathname = usePathname()
  const { region, set } = useOnboardingFlow()

  return (
    <OnboardingShell
      secondaryAction={
        <OnboardingBack onClick={() => router.push(prevPath(pathname)!)} />
      }
      primaryAction={
        <OnboardingContinue onClick={() => router.push(nextPath(pathname)!)} />
      }
    >
      <OptionGrid
        columns={1}
        options={REGION_OPTIONS}
        selected={(id) => region === id}
        onSelect={(id) => set({ region: id })}
      />
    </OnboardingShell>
  )
}
