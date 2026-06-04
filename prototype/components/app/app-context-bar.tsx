"use client"

import * as React from "react"
import Link from "next/link"
import { usePathname } from "next/navigation"
import {
  RiBarChartLine,
  RiBook2Line,
  RiFileList3Line,
  RiFileSearchLine,
  RiFileTextLine,
  RiFolder3Line,
  RiLayoutGrid2Line,
  RiNodeTree,
  RiSettings3Line,
  RiShieldKeyholeLine,
  RiSparkling2Line,
  RiTeamLine,
} from "@remixicon/react"

import { StylePicker } from "@/components/brand/style-picker"
import { ChromeActionHost } from "@/components/app/chrome"
import { pathOf } from "@/lib/mock-book"

type Crumb = { label: string; href?: string }

const SECTION: Record<string, { label: string; icon: React.ElementType }> = {
  ask: { label: "Ask", icon: RiSparkling2Line },
  book: { label: "Book", icon: RiBook2Line },
  sources: { label: "Sources", icon: RiLayoutGrid2Line },
  entities: { label: "Entities", icon: RiNodeTree },
  review: { label: "Review", icon: RiFileSearchLine },
  members: { label: "Members", icon: RiTeamLine },
  access: { label: "Access", icon: RiShieldKeyholeLine },
  audit: { label: "Audit log", icon: RiFileList3Line },
  usage: { label: "Usage", icon: RiBarChartLine },
  settings: { label: "Settings", icon: RiSettings3Line },
  account: { label: "Account", icon: RiSettings3Line },
}

const LEVEL_ICON: Record<string, React.ElementType> = {
  chapter: RiBook2Line,
  topic: RiFolder3Line,
  subtopic: RiFileTextLine,
}

function resolve(pathname: string): { icon: React.ElementType; crumbs: Crumb[] } {
  const segs = pathname.split("/").filter(Boolean)
  if (segs.length === 0) return { icon: RiBook2Line, crumbs: [] }

  if (segs[0] === "book") {
    if (segs.length === 1) return { icon: RiBook2Line, crumbs: [{ label: "Book" }] }
    const trail = pathOf(segs[1])
    const leaf = trail[trail.length - 1]
    return {
      icon: LEVEL_ICON[leaf?.level ?? "subtopic"] ?? RiFileTextLine,
      crumbs: [
        { label: "Book", href: "/book" },
        ...trail.map((e, i) => ({
          label: e.title,
          href: i < trail.length - 1 ? `/book/${e.id}` : undefined,
        })),
      ],
    }
  }

  if (segs[0] === "entities" && segs[1] === "ambiguity") {
    return {
      icon: RiNodeTree,
      crumbs: [{ label: "Entities", href: "/entities" }, { label: "Ambiguity queue" }],
    }
  }

  const s = SECTION[segs[0]]
  return s ? { icon: s.icon, crumbs: [{ label: s.label }] } : { icon: RiBook2Line, crumbs: [] }
}

export function AppContextBar() {
  const pathname = usePathname()
  const { icon: Icon, crumbs } = resolve(pathname)

  return (
    <header className="flex h-11 shrink-0 items-center gap-2 border-b border-border bg-background pl-3 pr-2">
      <Icon className="size-4 shrink-0 text-muted-foreground" />
      <nav className="flex min-w-0 flex-1 items-center gap-1.5 text-[13px]">
        {crumbs.map((c, i) => (
          <React.Fragment key={`${c.label}-${i}`}>
            {i > 0 && (
              <span aria-hidden className="shrink-0 text-border">
                /
              </span>
            )}
            {c.href ? (
              <Link
                href={c.href}
                className="max-w-[180px] truncate text-muted-foreground transition-colors hover:text-foreground"
              >
                {c.label}
              </Link>
            ) : (
              <span className="max-w-[280px] truncate font-medium text-foreground">
                {c.label}
              </span>
            )}
          </React.Fragment>
        ))}
      </nav>

      <ChromeActionHost className="flex shrink-0 items-center gap-1.5" />
      <StylePicker />
    </header>
  )
}
