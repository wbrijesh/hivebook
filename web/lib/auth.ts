"use client"

import { UserManager, WebStorageStateStore } from "oidc-client-ts"

// Browser-side OIDC (Authorization Code + PKCE) against ZITADEL. Config is baked
// at build time from NEXT_PUBLIC_* env (see Dockerfile build args). The browser
// trusts the mkcert CA via the macOS keychain, so the redirects and token
// exchange to https://id.hivebook.localhost just work.

let manager: UserManager | null = null

// Required public config, baked at build time. A missing value here is a
// misconfigured build — fail loudly with the name rather than letting `undefined`
// surface later as a cryptic OIDC error.
function requireEnv(name: string, value: string | undefined): string {
  if (!value) throw new Error(`missing required env ${name}`)
  return value
}

export function userManager(): UserManager {
  if (manager) return manager

  const issuer = requireEnv(
    "NEXT_PUBLIC_OIDC_ISSUER",
    process.env.NEXT_PUBLIC_OIDC_ISSUER
  )
  const clientId = requireEnv(
    "NEXT_PUBLIC_OIDC_CLIENT_ID",
    process.env.NEXT_PUBLIC_OIDC_CLIENT_ID
  )
  const projectId = process.env.NEXT_PUBLIC_OIDC_PROJECT_ID

  const scopes = ["openid", "profile", "email"]
  // Reserved ZITADEL scope that exposes the user's organization (resource owner)
  // via userinfo — that org is our tenant (design-doc 0005).
  scopes.push("urn:zitadel:iam:user:resourceowner")
  // Reserved ZITADEL scope that adds the project (and its api app) to the access
  // token audience, so the Go API can validate the token as its intended audience.
  if (projectId) scopes.push(`urn:zitadel:iam:org:project:id:${projectId}:aud`)

  manager = new UserManager({
    authority: issuer,
    client_id: clientId,
    redirect_uri: `${window.location.origin}/callback`,
    post_logout_redirect_uri: `${window.location.origin}/`,
    response_type: "code",
    scope: scopes.join(" "),
    userStore: new WebStorageStateStore({ store: window.localStorage }),
    // Renew the access token in the background before it expires (hidden iframe →
    // /silent-renew). The Connect transport also renews once on a 401 as a
    // safety net (lib/api/client.ts).
    automaticSilentRenew: true,
    silent_redirect_uri: `${window.location.origin}/silent-renew`,
  })
  return manager
}

// accessToken is the current OIDC access token, or null when signed out / expired.
// The Connect transport's auth interceptor reads it; nothing else attaches tokens
// (standards/frontend/data-fetching).
export async function accessToken(): Promise<string | null> {
  const u = await userManager().getUser()
  if (!u || u.expired) return null
  return u.access_token
}

export const API_BASE = (process.env.NEXT_PUBLIC_API_BASE as string) || ""
