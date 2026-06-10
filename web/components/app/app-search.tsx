"use client"

import * as React from "react"
import { useRouter } from "next/navigation"
import { Command as Cmdk } from "cmdk"
import {
  RiBarChartLine,
  RiBook2Line,
  RiCornerDownLeftLine,
  RiFileList3Line,
  RiFileSearchLine,
  RiLayoutGrid2Line,
  RiNodeTree,
  RiSearchLine,
  RiSettings3Line,
  RiShieldKeyholeLine,
  RiSparkling2Line,
  RiTeamLine,
} from "@remixicon/react"

import { Kbd } from "@/components/ui/kbd"
import { cn } from "@/lib/utils"

// A keyboard jumper for the real navigation destinations; "/" focuses it from
// anywhere. There's no corpus content to search yet (Book/Sources/Entities aren't
// built), so it jumps to sections — it gains content search when those land.
const NAV: { label: string; href: string; icon: React.ElementType }[] = [
  { label: "Ask", href: "/ask", icon: RiSparkling2Line },
  { label: "Book", href: "/book", icon: RiBook2Line },
  { label: "Sources", href: "/sources", icon: RiLayoutGrid2Line },
  { label: "Entities", href: "/entities", icon: RiNodeTree },
  { label: "Review", href: "/review", icon: RiFileSearchLine },
  { label: "Members", href: "/members", icon: RiTeamLine },
  { label: "Access", href: "/access", icon: RiShieldKeyholeLine },
  { label: "Audit log", href: "/audit", icon: RiFileList3Line },
  { label: "Usage", href: "/usage", icon: RiBarChartLine },
  { label: "Settings", href: "/settings", icon: RiSettings3Line },
]

export function AppSearch() {
  const router = useRouter()
  const rootRef = React.useRef<HTMLDivElement>(null)
  const inputRef = React.useRef<HTMLInputElement>(null)
  const [q, setQ] = React.useState("")
  const [open, setOpen] = React.useState(false)

  const query = q.trim()
  const showPanel = open && query.length > 0

  // "/" focuses search from anywhere — unless the user is already typing.
  React.useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key !== "/" || e.metaKey || e.ctrlKey || e.altKey) return
      const t = e.target as HTMLElement | null
      const typing =
        !!t &&
        (t.tagName === "INPUT" ||
          t.tagName === "TEXTAREA" ||
          t.isContentEditable)
      if (typing) return
      e.preventDefault()
      inputRef.current?.focus()
      setOpen(true)
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [])

  // Dismiss the panel on an outside click (the input keeps its text).
  React.useEffect(() => {
    function onDown(e: MouseEvent) {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false)
    }
    window.addEventListener("mousedown", onDown)
    return () => window.removeEventListener("mousedown", onDown)
  }, [])

  function go(href: string) {
    setOpen(false)
    setQ("")
    inputRef.current?.blur()
    router.push(href)
  }

  return (
    <Cmdk
      ref={rootRef}
      loop
      label="Jump to a section"
      className="relative"
      onKeyDown={(e) => {
        if (e.key === "Escape") {
          setQ("")
          setOpen(false)
          inputRef.current?.blur()
        }
      }}
    >
      <div
        className={cn(
          "flex h-7 w-56 items-center gap-2 rounded-md border border-input bg-background px-2.5 text-[13px] transition-colors lg:w-72",
          "focus-within:border-ring focus-within:ring-2 focus-within:ring-ring/25"
        )}
      >
        <RiSearchLine className="size-4 shrink-0 text-muted-foreground" />
        <Cmdk.Input
          ref={inputRef}
          value={q}
          onValueChange={(v) => {
            setQ(v)
            setOpen(true)
          }}
          onFocus={() => setOpen(true)}
          placeholder="Jump to a section…"
          className="min-w-0 flex-1 bg-transparent text-foreground outline-none placeholder:text-muted-foreground"
        />
        {!q && (
          <Kbd className="shrink-0 border border-border bg-transparent text-muted-foreground">
            /
          </Kbd>
        )}
      </div>

      {showPanel && (
        <div className="absolute top-[calc(100%+6px)] right-0 z-50 w-[440px] animate-in overflow-hidden rounded-lg bg-popover text-popover-foreground shadow-md ring-1 ring-foreground/10 duration-100 fade-in-0 zoom-in-95">
          <Cmdk.List className="no-scrollbar max-h-[min(64vh,420px)] overflow-y-auto p-1.5">
            <Cmdk.Empty className="px-3 py-8 text-center text-[13px] text-muted-foreground">
              No matches for “{query}”.
            </Cmdk.Empty>

            <Cmdk.Group
              className="[&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:pt-2 [&_[cmdk-group-heading]]:pb-1 [&_[cmdk-group-heading]]:text-[10px] [&_[cmdk-group-heading]]:font-medium [&_[cmdk-group-heading]]:tracking-wide [&_[cmdk-group-heading]]:text-muted-foreground/70 [&_[cmdk-group-heading]]:uppercase"
              heading="Go to"
            >
              {NAV.map((n) => {
                const Icon = n.icon
                return (
                  <Cmdk.Item
                    key={n.href}
                    value={n.label}
                    onSelect={() => go(n.href)}
                    className="flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 outline-none select-none data-[selected=true]:bg-muted"
                  >
                    <Icon className="size-4 shrink-0 text-muted-foreground" />
                    <span className="min-w-0 flex-1 truncate text-[13px] font-medium text-foreground">
                      {n.label}
                    </span>
                  </Cmdk.Item>
                )
              })}
            </Cmdk.Group>
          </Cmdk.List>

          <div className="flex items-center justify-between border-t border-border px-3 py-1.5 text-[11px] text-muted-foreground">
            <span className="inline-flex items-center gap-1.5">
              <RiCornerDownLeftLine className="size-3" /> open
            </span>
            <span className="inline-flex items-center gap-1.5">
              <Kbd className="border border-border bg-transparent">esc</Kbd>{" "}
              dismiss
            </span>
          </div>
        </div>
      )}
    </Cmdk>
  )
}
