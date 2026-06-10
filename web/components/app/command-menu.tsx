"use client"

import * as React from "react"
import { useRouter } from "next/navigation"
import { RiArrowRightUpLine } from "@remixicon/react"

import {
  Command,
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"

// A keyboard jumper (⌘K) for the real navigation destinations. No content search
// yet — Book/Ask aren't built, so there's nothing real to search inside them.
const NAV: { label: string; href: string }[] = [
  { label: "Ask", href: "/ask" },
  { label: "Book", href: "/book" },
  { label: "Sources", href: "/sources" },
  { label: "Entities", href: "/entities" },
  { label: "Review", href: "/review" },
  { label: "Members", href: "/members" },
  { label: "Access", href: "/access" },
  { label: "Audit log", href: "/audit" },
  { label: "Usage", href: "/usage" },
  { label: "Settings", href: "/settings" },
]

const CommandMenuContext = React.createContext<{ open: () => void } | null>(
  null
)

export function useCommandMenu() {
  const ctx = React.useContext(CommandMenuContext)
  if (!ctx)
    throw new Error("useCommandMenu must be used within CommandMenuProvider")
  return ctx
}

export function CommandMenuProvider({
  children,
}: {
  children: React.ReactNode
}) {
  const router = useRouter()
  const [open, setOpen] = React.useState(false)

  React.useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault()
        setOpen((v) => !v)
      }
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [])

  function go(href: string) {
    setOpen(false)
    router.push(href)
  }

  return (
    <CommandMenuContext.Provider value={{ open: () => setOpen(true) }}>
      {children}
      <CommandDialog
        open={open}
        onOpenChange={setOpen}
        className="max-w-[560px]"
      >
        <Command>
          <CommandInput placeholder="Jump to a section…" />
          <CommandList>
            <CommandEmpty>No matches.</CommandEmpty>
            <CommandGroup heading="Go to">
              {NAV.map((n) => (
                <CommandItem
                  key={n.href}
                  value={`go ${n.label}`}
                  onSelect={() => go(n.href)}
                >
                  <RiArrowRightUpLine className="size-4 text-muted-foreground" />
                  <span className="flex-1 truncate">{n.label}</span>
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </CommandDialog>
    </CommandMenuContext.Provider>
  )
}
