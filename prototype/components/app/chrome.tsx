"use client"

import * as React from "react"
import { createPortal } from "react-dom"

// Lets a page render controls into the context bar (the dense, page-specific
// action cluster) without the layout knowing what they are. The context bar
// registers a host element; pages portal their actions into it.
const HostCtx = React.createContext<HTMLElement | null>(null)
const SetHostCtx = React.createContext<(el: HTMLElement | null) => void>(() => {})

export function ChromeProvider({ children }: { children: React.ReactNode }) {
  const [host, setHost] = React.useState<HTMLElement | null>(null)
  return (
    <SetHostCtx.Provider value={setHost}>
      <HostCtx.Provider value={host}>{children}</HostCtx.Provider>
    </SetHostCtx.Provider>
  )
}

/** The context bar mounts this where page actions should appear. */
export function ChromeActionHost({ className }: { className?: string }) {
  const setHost = React.useContext(SetHostCtx)
  return <div ref={setHost} className={className} />
}

/** Pages wrap their context-bar controls in this. */
export function ChromeActions({ children }: { children: React.ReactNode }) {
  const host = React.useContext(HostCtx)
  if (!host) return null
  return createPortal(children, host)
}
