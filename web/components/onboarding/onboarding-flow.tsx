"use client"

import * as React from "react"

// Carries the user's answers across the onboarding steps so the flow feels
// continuous (e.g. the org name set on step 1 reads back on the summary). Lives
// in the onboarding layout, which persists across step navigations. Resets on a
// hard refresh — fine, because the layout re-seeds it from GET /api/tenant.
type OnboardingState = {
  orgName: string
  orgSize: string | null
  useCases: string[]
  useCaseOther: string // free-text when "other" is selected
  region: string
}

type OnboardingFlowValue = OnboardingState & {
  set: (patch: Partial<OnboardingState>) => void
  toggleUseCase: (id: string) => void
}

const OnboardingFlowContext = React.createContext<OnboardingFlowValue | null>(
  null
)

export function OnboardingFlowProvider({
  initial,
  children,
}: {
  initial: Partial<OnboardingState>
  children: React.ReactNode
}) {
  const [state, setState] = React.useState<OnboardingState>({
    orgName: "",
    orgSize: null,
    useCases: [],
    useCaseOther: "",
    region: "", // unchosen — region is write-once (ADR-0014), so never pre-select
    ...initial,
  })

  const value = React.useMemo<OnboardingFlowValue>(() => {
    const set = (patch: Partial<OnboardingState>) =>
      setState((s) => ({ ...s, ...patch }))
    return {
      ...state,
      set,
      toggleUseCase: (id) =>
        setState((s) => ({
          ...s,
          useCases: s.useCases.includes(id)
            ? s.useCases.filter((x) => x !== id)
            : [...s.useCases, id],
        })),
    }
  }, [state])

  return (
    <OnboardingFlowContext.Provider value={value}>
      {children}
    </OnboardingFlowContext.Provider>
  )
}

export function useOnboardingFlow() {
  const ctx = React.useContext(OnboardingFlowContext)
  if (!ctx)
    throw new Error(
      "useOnboardingFlow must be used within OnboardingFlowProvider"
    )
  return ctx
}
