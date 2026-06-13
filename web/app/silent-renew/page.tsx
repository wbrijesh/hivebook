"use client"

import { useEffect } from "react"

import { userManager } from "@/lib/auth"

// The hidden-iframe target for oidc-client-ts silent renew. It completes the
// prompt=none token exchange and posts the result back to the parent; nothing is
// ever shown here.
export default function SilentRenew() {
  useEffect(() => {
    userManager()
      .signinSilentCallback()
      .catch(() => {})
  }, [])
  return null
}
