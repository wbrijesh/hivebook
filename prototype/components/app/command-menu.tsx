"use client"

import * as React from "react"
import { useRouter } from "next/navigation"
import { RiSparkling2Line, RiArrowRightUpLine } from "@remixicon/react"

import {
  Command,
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "@/components/ui/command"
import { BOOK } from "@/lib/mock-book"

const NAV: { label: string; href: string }[] = [
  { label: "Ask", href: "/ask" },
  { label: "Book", href: "/book" },
  { label: "Sources", href: "/sources" },
  { label: "Members", href: "/members" },
  { label: "Access", href: "/access" },
  { label: "Entities", href: "/entities" },
  { label: "Audit log", href: "/audit" },
  { label: "Usage", href: "/usage" },
  { label: "Settings", href: "/settings" },
]

const CommandMenuContext = React.createContext<{ open: () => void } | null>(null)

export function useCommandMenu() {
  const ctx = React.useContext(CommandMenuContext)
  if (!ctx) throw new Error("useCommandMenu must be used within CommandMenuProvider")
  return ctx
}

export function CommandMenuProvider({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const [open, setOpen] = React.useState(false)
  const [q, setQ] = React.useState("")

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
    setQ("")
    router.push(href)
  }

  const query = q.trim()

  return (
    <CommandMenuContext.Provider value={{ open: () => setOpen(true) }}>
      {children}
      <CommandDialog open={open} onOpenChange={setOpen} className="max-w-[560px]">
        {/* cmdk does the filtering/keyboard-matching; the Ask action is
            forceMount so it survives the filter and is always available. */}
        <Command>
          <CommandInput
            value={q}
            onValueChange={setQ}
            placeholder="Ask a question, or jump to…"
          />
          <CommandList>
            <CommandEmpty>No matches.</CommandEmpty>

            {query && (
              <CommandGroup heading="Ask">
                <CommandItem
                  forceMount
                  value={`ask ${query}`}
                  onSelect={() => go(`/ask?q=${encodeURIComponent(query)}`)}
                >
                  <RiSparkling2Line className="size-4 text-muted-foreground" />
                  <span className="flex-1 truncate">Ask “{query}”</span>
                </CommandItem>
              </CommandGroup>
            )}

            <CommandGroup heading="Jump to">
              {BOOK.map((e) => (
                <CommandItem
                  key={e.id}
                  value={`entry ${e.title}`}
                  onSelect={() => go(`/book/${e.id}`)}
                >
                  <span className="flex-1 truncate">{e.title}</span>
                  <span className="text-[12px] capitalize text-muted-foreground">
                    {e.level}
                  </span>
                </CommandItem>
              ))}
            </CommandGroup>

            <CommandSeparator />
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
