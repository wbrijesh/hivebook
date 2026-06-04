"use client"

import { useRouter, usePathname } from "next/navigation"

import {
  OnboardingShell,
  OnboardingBack,
  OnboardingContinue,
} from "@/components/onboarding/onboarding-shell"
import { OptionGrid } from "@/components/onboarding/onboarding-form"
import { useOnboardingFlow } from "@/components/onboarding/onboarding-flow"
import { useCases } from "@/lib/mock-data"
import { nextPath, prevPath } from "@/lib/onboarding-steps"

export default function UseCasePage() {
  const router = useRouter()
  const pathname = usePathname()
  const { useCases: selected, toggleUseCase } = useOnboardingFlow()

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
        columns={2}
        options={useCases}
        selected={(id) => selected.includes(id)}
        onSelect={toggleUseCase}
      />
    </OnboardingShell>
  )
}
