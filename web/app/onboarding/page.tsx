import { redirect } from "next/navigation"

// /onboarding has no content of its own — it enters at the first step.
export default function OnboardingIndex() {
  redirect("/onboarding/organization")
}
