// The onboarding flow, in order. Single source of truth for the step headers,
// the quiet progress fill, and back/next navigation. Keep it short — every
// step here must earn its place (important AND not obtainable another way).

export type OnboardingStep = {
  slug: string
  /** Left-aligned step heading. */
  title: string
  /** One line stating why we ask — the value that makes the step worth doing. */
  why?: string
}

export const ONBOARDING_STEPS: OnboardingStep[] = [
  {
    slug: "organization",
    title: "Your organization",
    why: "Names your workspace and scopes what we bring in.",
  },
  {
    slug: "use-case",
    title: "What you'll use it for",
    why: "These areas become the first chapters of your brain.",
  },
  {
    slug: "region",
    title: "Where your data lives",
    why: "The storage region is fixed after setup.",
  },
  { slug: "complete", title: "You're all set" },
]

const BASE = "/onboarding"

function slugFromPath(pathname: string): string {
  return pathname.split("/").filter(Boolean).pop() ?? ""
}

export function stepIndex(pathname: string): number {
  return ONBOARDING_STEPS.findIndex((s) => s.slug === slugFromPath(pathname))
}

export function currentStep(pathname: string): OnboardingStep | undefined {
  return ONBOARDING_STEPS[stepIndex(pathname)]
}

/** 0–1 fill for the progress line; advances one step at a time. */
export function progress(pathname: string): number {
  const i = stepIndex(pathname)
  if (i < 0) return 0
  return (i + 1) / ONBOARDING_STEPS.length
}

export function nextPath(pathname: string): string | null {
  const i = stepIndex(pathname)
  const next = i >= 0 ? ONBOARDING_STEPS[i + 1] : undefined
  return next ? `${BASE}/${next.slug}` : null
}

export function prevPath(pathname: string): string | null {
  const i = stepIndex(pathname)
  const prev = i > 0 ? ONBOARDING_STEPS[i - 1] : undefined
  return prev ? `${BASE}/${prev.slug}` : null
}
