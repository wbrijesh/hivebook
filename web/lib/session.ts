"use client"

import { Code, ConnectError } from "@connectrpc/connect"
import { useQuery } from "@connectrpc/connect-query"

import { getSession } from "@/lib/gen/hivebook/tenant/v1/tenant-TenantService_connectquery"

// useSession is the single source for "who am I and which workspace am I in" —
// one query, deduplicated by TanStack Query across every consumer (the gate, the
// spine, the onboarding chrome). It replaces the old /api/me + /api/tenant
// fetches and their duplicate call sites (design-doc 0006).
export function useSession() {
  return useQuery(getSession, {})
}

// isAuthError marks the only failures that should end the session: the token was
// rejected (Unauthenticated) or the caller lacks access (PermissionDenied).
// Everything else is transient and retryable — never a sign-out.
export function isAuthError(error: unknown): boolean {
  return (
    error instanceof ConnectError &&
    (error.code === Code.Unauthenticated ||
      error.code === Code.PermissionDenied)
  )
}
