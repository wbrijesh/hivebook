"use client"

import { UserManager, WebStorageStateStore } from "oidc-client-ts"

// Browser-side OIDC (Authorization Code + PKCE) against ZITADEL. Config is baked
// at build time from NEXT_PUBLIC_* env (see Dockerfile build args). The browser
// trusts the mkcert CA via the macOS keychain, so the redirects and token
// exchange to https://id.hivebook.localhost just work.

let manager: UserManager | null = null

export function userManager(): UserManager {
  if (manager) return manager

  const issuer = process.env.NEXT_PUBLIC_OIDC_ISSUER as string
  const clientId = process.env.NEXT_PUBLIC_OIDC_CLIENT_ID as string
  const projectId = process.env.NEXT_PUBLIC_OIDC_PROJECT_ID

  const scopes = ["openid", "profile", "email"]
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
  })
  return manager
}

export const API_BASE = (process.env.NEXT_PUBLIC_API_BASE as string) || ""
