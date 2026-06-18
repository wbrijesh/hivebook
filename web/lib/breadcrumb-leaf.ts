"use client"

import { useEffect, useSyncExternalStore } from "react"

// A tiny external store for the breadcrumb's leaf label. A detail page knows its
// own name (a connection's account, a file's title); the context bar doesn't —
// and shouldn't fetch it. The page publishes its name here while mounted and the
// breadcrumb reads it. An external store (not React state) so publishing from an
// effect doesn't trip the set-state-in-effect rule, and so the context bar only
// re-renders when the label actually changes.
let leaf: string | null = null
const listeners = new Set<() => void>()

function setLeaf(label: string | null) {
  if (leaf === label) return
  leaf = label
  for (const notify of listeners) notify()
}

// useBreadcrumbLeaf returns the currently-published leaf label, or null. SSR and
// first client render both see null, so there's no hydration mismatch — the real
// name arrives after the page's effect runs.
export function useBreadcrumbLeaf(): string | null {
  return useSyncExternalStore(
    (cb) => {
      listeners.add(cb)
      return () => listeners.delete(cb)
    },
    () => leaf,
    () => null
  )
}

// usePublishBreadcrumbLeaf sets the breadcrumb's leaf to a page's real name while
// it's mounted, clearing it on unmount so a stale name never lingers on the next
// page. Pass undefined while the name is still loading (falls back to the generic
// label).
export function usePublishBreadcrumbLeaf(label: string | null | undefined) {
  useEffect(() => {
    setLeaf(label ?? null)
    return () => setLeaf(null)
  }, [label])
}
