import { userManager, API_BASE } from "@/lib/auth"

// The tenant resource as returned by the Go API (design-doc 0005). `onboarded`
// drives the app/onboarding gate.
export type Tenant = {
  id: string
  name: string | null
  size: string | null
  region: string | null
  useCases: string[]
  useCaseOther: string | null
  onboarded: boolean
}

// The authenticated session: who the caller is and which workspace they're in.
// Both are resolved server-side (token/userinfo + DB) — the chrome never derives
// identity from client state (design-doc 0005).
export type SessionUser = { id: string; name: string; email: string }
export type Session = { user: SessionUser; tenant: Tenant }

export type OnboardingPayload = {
  name: string
  size: string
  region: string
  useCases: string[]
  useCaseOther: string
}

async function authHeader(): Promise<Record<string, string> | null> {
  const u = await userManager().getUser()
  if (!u || u.expired) return null
  return { Authorization: `Bearer ${u.access_token}` }
}

// GET /api/me — the authoritative session (identity + tenant), resolved entirely
// server-side. The chrome reads name/email/workspace from here, never from the
// OIDC profile in local state. Returns null when not signed in.
export async function fetchMe(): Promise<Session | null> {
  const h = await authHeader()
  if (!h) return null
  const res = await fetch(`${API_BASE}/api/me`, { headers: h })
  if (!res.ok) throw new Error(`session lookup failed (${res.status})`)
  return (await res.json()) as Session
}

// GET /api/tenant — lazily creates the tenant row server-side. Returns null when
// not signed in.
export async function fetchTenant(): Promise<Tenant | null> {
  const h = await authHeader()
  if (!h) return null
  const res = await fetch(`${API_BASE}/api/tenant`, { headers: h })
  if (!res.ok) throw new Error(`tenant lookup failed (${res.status})`)
  return (await res.json()) as Tenant
}

// POST /api/tenant/onboarding — idempotent; region is write-once server-side.
export async function submitOnboarding(
  body: OnboardingPayload
): Promise<Tenant> {
  const h = await authHeader()
  if (!h) throw new Error("not signed in")
  const res = await fetch(`${API_BASE}/api/tenant/onboarding`, {
    method: "POST",
    headers: { ...h, "Content-Type": "application/json" },
    body: JSON.stringify(body),
  })
  if (!res.ok)
    throw new Error(`onboarding failed (${res.status}): ${await res.text()}`)
  return (await res.json()) as Tenant
}
