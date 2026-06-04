"use client"

import * as React from "react"

// Carries the small amount of state that must follow a user across the auth
// funnel so the flow feels like it's thinking with them: the email they typed,
// and the sign-in method they last used. Lives in the (auth) layout, which
// persists across navigations within the group (a hard refresh resets it,
// which is fine — screens fall back to a sensible default).
type AuthFlowValue = {
  /** The work email the user entered on the front door. */
  email: string
  setEmail: (email: string) => void
  /** The id of the last method used ("email" | provider id), persisted. */
  lastMethod: string | null
  setLastMethod: (method: string) => void
}

const LAST_METHOD_KEY = "trenches-last-method"

const AuthFlowContext = React.createContext<AuthFlowValue | null>(null)

export function AuthFlowProvider({ children }: { children: React.ReactNode }) {
  const [email, setEmail] = React.useState("")
  const [lastMethod, setLastMethodState] = React.useState<string | null>(null)

  React.useEffect(() => {
    try {
      const m = localStorage.getItem(LAST_METHOD_KEY)
      if (m) setLastMethodState(m)
    } catch {
      // ignore
    }
  }, [])

  const setLastMethod = React.useCallback((method: string) => {
    setLastMethodState(method)
    try {
      localStorage.setItem(LAST_METHOD_KEY, method)
    } catch {
      // ignore
    }
  }, [])

  const value = React.useMemo<AuthFlowValue>(
    () => ({ email, setEmail, lastMethod, setLastMethod }),
    [email, lastMethod, setLastMethod]
  )

  return (
    <AuthFlowContext.Provider value={value}>
      {children}
    </AuthFlowContext.Provider>
  )
}

export function useAuthFlow() {
  const ctx = React.useContext(AuthFlowContext)
  if (!ctx) throw new Error("useAuthFlow must be used within AuthFlowProvider")
  return ctx
}
