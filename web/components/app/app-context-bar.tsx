"use client"

import * as React from "react"
import Link from "next/link"
import { usePathname } from "next/navigation"
import { RiBook2Line, RiNodeTree, RiSettings3Line } from "@remixicon/react"

import { AppSearch } from "@/components/app/app-search"
import { navByKey } from "@/lib/nav"

type Crumb = { label: string; href?: string }

// Primary sections come from the shared nav registry; secondary surfaces that
// aren't in the spine (e.g. account) live here.
const EXTRA: Record<string, { label: string; icon: React.ElementType }> = {
  account: { label: "Account", icon: RiSettings3Line },
}

// Top-level breadcrumb only. Deeper trails (e.g. inside Book) return once those
// features ship with real data.
function resolve(pathname: string): {
  icon: React.ElementType
  crumbs: Crumb[]
} {
  const segs = pathname.split("/").filter(Boolean)
  if (segs.length === 0) return { icon: RiBook2Line, crumbs: [] }

  if (segs[0] === "entities" && segs[1] === "ambiguity") {
    return {
      icon: RiNodeTree,
      crumbs: [
        { label: "Entities", href: "/entities" },
        { label: "Ambiguity queue" },
      ],
    }
  }

  const s = navByKey[segs[0]] ?? EXTRA[segs[0]]
  return s
    ? { icon: s.icon, crumbs: [{ label: s.label }] }
    : { icon: RiBook2Line, crumbs: [] }
}

export function AppContextBar() {
  const pathname = usePathname()
  const { icon: Icon, crumbs } = resolve(pathname)

  return (
    <header className="flex h-11 shrink-0 items-center gap-2 border-b border-border bg-background pr-2 pl-3">
      <Icon className="size-4 shrink-0 text-muted-foreground" />
      <nav
        aria-label="Breadcrumb"
        className="flex min-w-0 flex-1 items-center gap-1.5 text-[13px]"
      >
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

      <AppSearch />
    </header>
  )
}
