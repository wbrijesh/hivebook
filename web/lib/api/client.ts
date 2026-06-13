"use client"

import { Code, ConnectError, type Interceptor } from "@connectrpc/connect"
import { createConnectTransport } from "@connectrpc/connect-web"

import { accessToken, userManager, API_BASE } from "@/lib/auth"

// The one transport for the Connect API. It owns the base URL, the bearer token,
// and the renew-on-401 policy — call sites never touch any of these
// (standards/frontend/data-fetching).

// Attach the OIDC access token to every request.
const auth: Interceptor = (next) => async (req) => {
  const token = await accessToken()
  if (token) req.header.set("Authorization", `Bearer ${token}`)
  return next(req)
}

// Single-flight token renewal. On a cold load many RPCs can 401 at once; only one
// silent renew should run, and the rest await it. A short cooldown stops a token
// that still 401s after renewal (e.g. a misconfigured audience) from triggering a
// renew on every request — the second failure then propagates instead.
let renewing: Promise<unknown> | null = null
let lastRenewMs = 0
const renewCooldownMs = 10_000

async function renewSession(): Promise<void> {
  if (Date.now() - lastRenewMs < renewCooldownMs) {
    throw new Error("token renewal on cooldown")
  }
  renewing ??= userManager()
    .signinSilent()
    .finally(() => {
      lastRenewMs = Date.now()
      renewing = null
    })
  await renewing
}

// If a request comes back Unauthenticated (token expired between renewals),
// silently renew once and retry; the retry re-runs `auth` with the fresh token.
// A second failure — or no session to renew — propagates the real error, which
// the session policy turns into a sign-out (lib/session.ts).
const renewOnce: Interceptor = (next) => async (req) => {
  try {
    return await next(req)
  } catch (err) {
    if (err instanceof ConnectError && err.code === Code.Unauthenticated) {
      try {
        await renewSession()
      } catch {
        throw err
      }
      return next(req)
    }
    throw err
  }
}

export const transport = createConnectTransport({
  baseUrl: API_BASE,
  // Outermost first: renewOnce wraps auth, so a retry picks up the new token.
  interceptors: [renewOnce, auth],
})
