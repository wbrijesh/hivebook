"use client"

import { createContext, type ReactNode, useContext, useState } from "react"
import { createPortal } from "react-dom"

// A portal slot in the application chrome — the context bar, just left of search —
// that pages publish their header actions into. Page-level actions (e.g. "Add
// connector") belong to the chrome, never the page body. The action stays in the
// page's own render tree (real state, handlers, lifecycle); only its rendered DOM
// is relocated into the bar via a portal.
//
// Split into value + setter contexts so the slot element (which only sets the
// node) doesn't re-render when the node changes.
const SlotNode = createContext<HTMLElement | null>(null)
const SetSlotNode = createContext<(el: HTMLElement | null) => void>(() => {})

export function ChromeActionsProvider({ children }: { children: ReactNode }) {
  const [node, setNode] = useState<HTMLElement | null>(null)
  return (
    <SetSlotNode.Provider value={setNode}>
      <SlotNode.Provider value={node}>{children}</SlotNode.Provider>
    </SetSlotNode.Provider>
  )
}

// ChromeActionsSlot is the portal target, rendered once in the context bar. It
// collapses (empty:hidden) when no page has published actions, so it adds no gap.
export function ChromeActionsSlot({ className }: { className?: string }) {
  const setNode = useContext(SetSlotNode)
  return <div ref={setNode} className={className} />
}

// ChromeActions renders its children into the chrome slot, nothing inline. A page
// drops this anywhere in its JSX to put actions in the bar.
export function ChromeActions({ children }: { children: ReactNode }) {
  const node = useContext(SlotNode)
  if (!node) return null
  return createPortal(children, node)
}
