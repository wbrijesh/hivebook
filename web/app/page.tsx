"use client"

import { useEffect, useState } from "react"
import type { User } from "oidc-client-ts"

import { userManager, API_BASE } from "@/lib/auth"
import { track } from "@/lib/telemetry"

// Intentionally unstyled — this exists to prove the end-to-end auth wiring
// (ZITADEL login -> token -> calling the protected Go API), not to look good.
export default function Page() {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)
  const [apiResult, setApiResult] = useState<string>("")

  useEffect(() => {
    track("page_view", { path: "/" })
    userManager()
      .getUser()
      .then(setUser)
      .finally(() => setLoading(false))
  }, [])

  function login() {
    track("login_start")
    void userManager().signinRedirect()
  }

  async function callApi() {
    const u = await userManager().getUser()
    if (!u) return
    const res = await fetch(`${API_BASE}/api/me`, {
      headers: { Authorization: `Bearer ${u.access_token}` },
    })
    track("api_call", { endpoint: "/api/me", status: res.status })
    const body = await res.text()
    setApiResult(`HTTP ${res.status}\n${body}`)
  }

  if (loading) return <main style={{ padding: 24 }}>Loading…</main>

  return (
    <main style={{ padding: 24, fontFamily: "sans-serif", maxWidth: 720 }}>
      <h1>Hivebook</h1>
      {!user ? (
        <button onClick={login}>Log in with ZITADEL</button>
      ) : (
        <div>
          <p>
            Signed in as <strong>{user.profile.preferred_username ?? user.profile.sub}</strong>
          </p>
          <p>
            <button onClick={callApi}>Call GET /api/me</button>{" "}
            <button onClick={() => userManager().signoutRedirect()}>Log out</button>
          </p>
          {apiResult && (
            <pre style={{ background: "#f4f4f4", padding: 12, whiteSpace: "pre-wrap" }}>{apiResult}</pre>
          )}
        </div>
      )}
    </main>
  )
}
