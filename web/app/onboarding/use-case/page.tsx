"use client"

import { useRouter, usePathname } from "next/navigation"

import {
  OnboardingShell,
  OnboardingBack,
  OnboardingContinue,
} from "@/components/onboarding/onboarding-shell"
import { Field, OptionGrid } from "@/components/onboarding/onboarding-form"
import { useOnboardingFlow } from "@/components/onboarding/onboarding-flow"
import { Input } from "@/components/ui/input"
import { useCases } from "@/lib/onboarding-options"
import { nextPath, prevPath } from "@/lib/onboarding-steps"

export default function UseCasePage() {
  const router = useRouter()
  const pathname = usePathname()
  const {
    useCases: selected,
    useCaseOther,
    toggleUseCase,
    set,
  } = useOnboardingFlow()

  const otherSelected = selected.includes("other")

  return (
    <OnboardingShell
      secondaryAction={
        <OnboardingBack onClick={() => router.push(prevPath(pathname)!)} />
      }
      primaryAction={
        <OnboardingContinue onClick={() => router.push(nextPath(pathname)!)} />
      }
    >
      <div className="space-y-4">
        <OptionGrid
          columns={2}
          options={useCases}
          selected={(id) => selected.includes(id)}
          onSelect={toggleUseCase}
        />

        {/* Picking "Something else" is only meaningful if we capture what it is. */}
        {otherSelected && (
          <Field label="Tell us what else" htmlFor="use-case-other">
            <Input
              id="use-case-other"
              data-autofocus
              value={useCaseOther}
              onChange={(e) => set({ useCaseOther: e.target.value })}
              placeholder="e.g. Field service procedures"
              className="h-11 text-[14px]"
            />
          </Field>
        )}
      </div>
    </OnboardingShell>
  )
}
