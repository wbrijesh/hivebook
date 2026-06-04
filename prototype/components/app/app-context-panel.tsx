"use client"

import * as React from "react"
import Link from "next/link"
import { usePathname } from "next/navigation"
import { RiArrowRightSLine, RiSearchLine } from "@remixicon/react"

import { childrenOf, entryStats, type BookEntry } from "@/lib/mock-book"
import { cn } from "@/lib/utils"

// The section-contextual panel. Today it carries the Book tree when the user is
// anywhere under /book; other sections render their own filters inline, so the
// panel simply collapses away.
export function AppContextPanel() {
  const pathname = usePathname()
  if (!pathname.startsWith("/book")) return null
  return <BookPanel pathname={pathname} />
}

function BookPanel({ pathname }: { pathname: string }) {
  const [q, setQ] = React.useState("")
  const query = q.trim().toLowerCase()

  return (
    <aside className="flex w-60 shrink-0 flex-col border-r border-border bg-background">
      <div className="flex items-center justify-between px-3 pb-1.5 pt-3">
        <span className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
          Book
        </span>
        <span className="font-mono text-[11px] text-muted-foreground">
          {childrenOf(null).length} ch
        </span>
      </div>

      <div className="px-2 pb-2">
        <div className="flex items-center gap-1.5 rounded-md border border-input bg-background px-2 py-1">
          <RiSearchLine className="size-3.5 text-muted-foreground" />
          <input
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Filter"
            className="min-w-0 flex-1 bg-transparent text-[13px] text-foreground placeholder:text-muted-foreground focus:outline-none"
          />
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-2 pb-3">
        {childrenOf(null).map((ch) => (
          <TreeNode key={ch.id} entry={ch} depth={0} pathname={pathname} query={query} />
        ))}
      </div>
    </aside>
  )
}

function matches(entry: BookEntry, query: string): boolean {
  if (!query) return true
  if (entry.title.toLowerCase().includes(query)) return true
  return childrenOf(entry.id).some((c) => matches(c, query))
}

function StatusDot({ status }: { status: BookEntry["status"] }) {
  if (status === "current") return null
  return (
    <span
      className={cn(
        "size-1.5 shrink-0 rounded-full",
        status === "stale" ? "bg-warning" : "animate-pulse bg-muted-foreground/50"
      )}
      title={status === "stale" ? "Refreshing" : "Building"}
    />
  )
}

function TreeNode({
  entry,
  depth,
  pathname,
  query,
}: {
  entry: BookEntry
  depth: number
  pathname: string
  query: string
}) {
  const kids = childrenOf(entry.id)
  const hasKids = kids.length > 0
  const active = pathname === `/book/${entry.id}`
  const [open, setOpen] = React.useState(depth < 1)
  const expanded = open || (!!query && hasKids)

  if (!matches(entry, query)) return null

  const count =
    entry.level === "chapter"
      ? entryStats(entry.id).topics
      : entry.level === "topic"
        ? entryStats(entry.id).subtopics
        : 0

  return (
    <div>
      <div
        className={cn(
          "group flex items-center gap-1 rounded-md pr-1.5 text-[13px] transition-colors",
          active
            ? "bg-muted font-medium text-foreground"
            : "text-muted-foreground hover:bg-muted hover:text-foreground"
        )}
        style={{ paddingLeft: `${depth * 12 + 2}px` }}
      >
        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          className={cn(
            "flex size-5 shrink-0 items-center justify-center text-muted-foreground",
            !hasKids && "invisible"
          )}
          aria-label={expanded ? "Collapse" : "Expand"}
        >
          <RiArrowRightSLine
            className={cn("size-3.5 transition-transform", expanded && "rotate-90")}
          />
        </button>
        <Link
          href={`/book/${entry.id}`}
          className="flex min-w-0 flex-1 items-center gap-2 py-1.5"
        >
          <span className="min-w-0 flex-1 truncate">{entry.title}</span>
          <StatusDot status={entry.status} />
          {count > 0 && (
            <span className="shrink-0 font-mono text-[11px] text-muted-foreground/70">
              {count}
            </span>
          )}
        </Link>
      </div>
      {hasKids && expanded && (
        <div>
          {kids.map((k) => (
            <TreeNode
              key={k.id}
              entry={k}
              depth={depth + 1}
              pathname={pathname}
              query={query}
            />
          ))}
        </div>
      )}
    </div>
  )
}
