"use client"

import * as React from "react"
import { useRouter } from "next/navigation"
import { Command as Cmdk } from "cmdk"
import {
  RiBook2Line,
  RiCornerDownLeftLine,
  RiFileTextLine,
  RiFolder3Line,
  RiNodeTree,
  RiSearchLine,
} from "@remixicon/react"

import { Kbd } from "@/components/ui/kbd"
import {
  BOOK,
  ENTITIES,
  SOURCES,
  pathOf,
  type BookEntry,
  type Entity,
  type EntryStatus,
  type Source,
  type SourceHealth,
} from "@/lib/mock-book"
import { cn } from "@/lib/utils"

// Corpus search — its own surface, distinct from the ⌘K command palette. The
// palette runs commands and jumps; this finds objects in the company brain and
// shows the state they carry. Lives in the context bar; "/" focuses it.

const LEVEL_ICON = {
  chapter: RiBook2Line,
  topic: RiFolder3Line,
  subtopic: RiFileTextLine,
} as const

// Status → dot color. Same vocabulary the rest of the app uses for object state.
const ENTRY_DOT: Record<EntryStatus, string> = {
  current: "bg-success",
  stale: "bg-warning",
  building: "bg-muted-foreground/40",
}

const SOURCE_DOT: Record<SourceHealth, string> = {
  synced: "bg-success",
  syncing: "bg-warning",
  failed: "bg-destructive",
  "auth-lapse": "bg-destructive",
}

function Dot({ className }: { className: string }) {
  return <span className={cn("size-1.5 shrink-0 rounded-full", className)} aria-hidden />
}

function trail(entry: BookEntry): string {
  return pathOf(entry.id)
    .slice(0, -1)
    .map((e) => e.title)
    .join(" / ")
}

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
        !!t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.isContentEditable)
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

  const entries = BOOK
  const sources = SOURCES
  const entities = ENTITIES

  return (
    <Cmdk
      ref={rootRef}
      loop
      label="Search the corpus"
      className="relative"
      // We drive panel visibility ourselves; let cmdk filter on the input value.
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
          placeholder="Search the book…"
          className="min-w-0 flex-1 bg-transparent text-foreground outline-none placeholder:text-muted-foreground"
        />
        {!q && (
          <Kbd className="shrink-0 border border-border bg-transparent text-muted-foreground">
            /
          </Kbd>
        )}
      </div>

      {showPanel && (
        <div className="absolute top-[calc(100%+6px)] right-0 z-50 w-[440px] overflow-hidden rounded-lg bg-popover text-popover-foreground shadow-md ring-1 ring-foreground/10 duration-100 animate-in fade-in-0 zoom-in-95">
          <Cmdk.List className="no-scrollbar max-h-[min(64vh,420px)] overflow-y-auto p-1.5">
            <Cmdk.Empty className="px-3 py-8 text-center text-[13px] text-muted-foreground">
              No matches for “{query}”.
            </Cmdk.Empty>

            <Group heading="Book">
              {entries.map((e) => (
                <BookRow key={e.id} entry={e} onSelect={() => go(`/book/${e.id}`)} />
              ))}
            </Group>

            <Group heading="Sources">
              {sources.map((s) => (
                <SourceRow key={s.id} source={s} onSelect={() => go("/sources")} />
              ))}
            </Group>

            <Group heading="Entities">
              {entities.map((en) => (
                <EntityRow key={en.id} entity={en} onSelect={() => go("/entities")} />
              ))}
            </Group>
          </Cmdk.List>

          <div className="flex items-center justify-between border-t border-border px-3 py-1.5 text-[11px] text-muted-foreground">
            <span className="inline-flex items-center gap-1.5">
              <RiCornerDownLeftLine className="size-3" /> open
            </span>
            <span className="inline-flex items-center gap-1.5">
              <Kbd className="border border-border bg-transparent">esc</Kbd> dismiss
            </span>
          </div>
        </div>
      )}
    </Cmdk>
  )
}

function Group({ heading, children }: { heading: string; children: React.ReactNode }) {
  return (
    <Cmdk.Group
      className="[&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:pt-2 [&_[cmdk-group-heading]]:pb-1 [&_[cmdk-group-heading]]:text-[10px] [&_[cmdk-group-heading]]:font-medium [&_[cmdk-group-heading]]:tracking-wide [&_[cmdk-group-heading]]:text-muted-foreground/70 [&_[cmdk-group-heading]]:uppercase"
      heading={heading}
    >
      {children}
    </Cmdk.Group>
  )
}

const rowClass =
  "flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 outline-none select-none data-[selected=true]:bg-muted"

function BookRow({ entry, onSelect }: { entry: BookEntry; onSelect: () => void }) {
  const Icon = LEVEL_ICON[entry.level]
  const path = trail(entry)
  return (
    <Cmdk.Item
      value={entry.id}
      keywords={[entry.title, entry.level, path, entry.summary ?? ""]}
      onSelect={onSelect}
      className={rowClass}
    >
      <Dot className={ENTRY_DOT[entry.status]} />
      <Icon className="size-4 shrink-0 text-muted-foreground" />
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="truncate text-[13px] font-medium text-foreground">
            {entry.title}
          </span>
          <span className="ml-auto shrink-0 text-[11px] capitalize text-muted-foreground">
            {entry.level}
          </span>
        </div>
        {(path || entry.lastRebuilt) && (
          <div className="truncate text-[11px] text-muted-foreground">
            {[path, entry.lastRebuilt ? `rebuilt ${entry.lastRebuilt}` : null]
              .filter(Boolean)
              .join(" · ")}
          </div>
        )}
      </div>
    </Cmdk.Item>
  )
}

function SourceRow({ source, onSelect }: { source: Source; onSelect: () => void }) {
  return (
    <Cmdk.Item
      value={source.id}
      keywords={[source.name, source.kind]}
      onSelect={onSelect}
      className={rowClass}
    >
      <span
        className="flex size-5 shrink-0 items-center justify-center rounded-sm text-[11px] font-semibold text-white"
        style={{ backgroundColor: source.color }}
        aria-hidden
      >
        {source.letter}
      </span>
      <span className="min-w-0 flex-1 truncate text-[13px] font-medium text-foreground">
        {source.name}
      </span>
      <Dot className={SOURCE_DOT[source.health]} />
      <span className="shrink-0 tabular-nums text-[11px] text-muted-foreground">
        {source.artifacts.toLocaleString()} artifacts
      </span>
    </Cmdk.Item>
  )
}

function EntityRow({ entity, onSelect }: { entity: Entity; onSelect: () => void }) {
  return (
    <Cmdk.Item
      value={entity.id}
      keywords={[entity.name, entity.type]}
      onSelect={onSelect}
      className={rowClass}
    >
      <RiNodeTree className="size-4 shrink-0 text-muted-foreground" />
      <span className="min-w-0 flex-1 truncate text-[13px] font-medium text-foreground">
        {entity.name}
      </span>
      <span className="shrink-0 text-[11px] capitalize text-muted-foreground">
        {entity.type}
      </span>
      <span className="shrink-0 tabular-nums text-[11px] text-muted-foreground">
        {entity.mentions}×
      </span>
    </Cmdk.Item>
  )
}
