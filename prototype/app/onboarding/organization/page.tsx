"use client"

import { useRouter, usePathname } from "next/navigation"

import {
  OnboardingShell,
  OnboardingContinue,
} from "@/components/onboarding/onboarding-shell"
import { Field, OptionGrid } from "@/components/onboarding/onboarding-form"
import { useOnboardingFlow } from "@/components/onboarding/onboarding-flow"
import { Input } from "@/components/ui/input"
import { companySizes } from "@/lib/mock-data"
import { nextPath } from "@/lib/onboarding-steps"

function slugify(s: string): string {
  return s
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
}

export default function OrganizationPage() {
  const router = useRouter()
  const pathname = usePathname()
  const { orgName, orgSize, set } = useOnboardingFlow()

  const slug = slugify(orgName) || "your-workspace"
  const canContinue = orgName.trim().length > 0

  return (
    <OnboardingShell
      primaryAction={
        <OnboardingContinue
          disabled={!canContinue}
          onClick={() => router.push(nextPath(pathname)!)}
        />
      }
    >
      <div className="space-y-6">
        <Field
          label="Organization name"
          htmlFor="org"
          hint={`Your workspace will live at trenches.run/${slug}`}
        >
          <Input
            id="org"
            data-autofocus
            value={orgName}
            onChange={(e) => set({ orgName: e.target.value })}
            placeholder="Acme Inc."
            className="h-11 text-[14px]"
          />
        </Field>

        <Field label="Company size">
          <OptionGrid
            columns={3}
            options={companySizes}
            selected={(id) => orgSize === id}
            onSelect={(id) => set({ orgSize: orgSize === id ? null : id })}
          />
        </Field>
      </div>
    </OnboardingShell>
  )
}
