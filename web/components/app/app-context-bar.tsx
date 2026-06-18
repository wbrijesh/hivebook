"use client"

import * as React from "react"
import Link from "next/link"
import { usePathname } from "next/navigation"

import { AppSearch } from "@/components/app/app-search"
import { ChromeActionsSlot } from "@/components/app/chrome-actions"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import { useBreadcrumbLeaf } from "@/lib/breadcrumb-leaf"
import { navByKey } from "@/lib/nav"

type Crumb = { label: string; href?: string; detail?: boolean }

// Labels for a section's leaf pages — id routes (e.g. /sources/<id>) don't carry a
// name in the URL, so we name the level generically until the page itself can.
const DETAIL_LABEL: Record<string, string> = {
  sources: "Source",
  files: "File",
  book: "Chapter",
  entities: "Entity",
}

// Secondary surfaces not in the spine registry.
const EXTRA: Record<string, { label: string; href: string }> = {
  account: { label: "Account", href: "/account" },
}

// buildCrumbs resolves the trail, always rooted at Home. Deeper trails return once
// those features ship with real data.
function buildCrumbs(pathname: string): Crumb[] {
  const segs = pathname.split("/").filter(Boolean)
  const crumbs: Crumb[] = [{ label: "Home", href: "/home" }]
  if (segs.length === 0 || segs[0] === "home") return crumbs

  const key = segs[0]
  const section = navByKey[key] ?? EXTRA[key]
  if (!section) return crumbs

  const hasDeeper = segs.length > 1
  crumbs.push({
    label: section.label,
    href: hasDeeper ? section.href : undefined,
  })

  if (key === "sources" && segs[1] === "new") {
    const deeper = segs.length > 2 // /sources/new/<connector>
    crumbs.push({
      label: "Add source",
      href: deeper ? "/sources/new" : undefined,
    })
    if (deeper) {
      // The connect page publishes the connector's name as the leaf.
      crumbs.push({ label: "Source", detail: true })
    }
  } else if (key === "entities" && segs[1] === "ambiguity") {
    crumbs.push({ label: "Ambiguity queue" })
  } else if (hasDeeper) {
    // The detail crumb carries the page-published name (filled in below). A sub-page
    // like /sources/<id>/manage nests one level under it: the detail crumb links to
    // the source and "Manage" becomes the leaf.
    const isManage = key === "sources" && segs[2] === "manage"
    crumbs.push({
      label: DETAIL_LABEL[key] ?? "Details",
      href: isManage ? `/sources/${segs[1]}` : undefined,
      detail: true,
    })
    if (isManage) crumbs.push({ label: "Manage" })
  }
  return crumbs
}

export function AppContextBar() {
  const pathname = usePathname()
  const leaf = useBreadcrumbLeaf()
  const crumbs = buildCrumbs(pathname)
  const last = crumbs.length - 1

  // Replace the generic detail label ("Source", "File") with the real name the page
  // published. The detail crumb isn't always the leaf — on /sources/<id>/manage it's
  // one level up (… / <source> / Manage) — so target the crumb flagged `detail`.
  const detailIdx = crumbs.findIndex((c) => c.detail)
  if (leaf && detailIdx >= 0) {
    crumbs[detailIdx] = { ...crumbs[detailIdx], label: leaf }
  }

  return (
    <header className="flex h-11 shrink-0 items-center gap-2 border-b border-border bg-background pr-2 pl-3">
      <Breadcrumb className="min-w-0 flex-1">
        <BreadcrumbList className="flex-nowrap text-[13px]">
          {crumbs.map((c, i) => (
            <React.Fragment key={i}>
              {i > 0 && <BreadcrumbSeparator />}
              <BreadcrumbItem className="min-w-0">
                {i !== last && c.href ? (
                  <BreadcrumbLink asChild>
                    <Link href={c.href} className="max-w-[200px] truncate">
                      {c.label}
                    </Link>
                  </BreadcrumbLink>
                ) : (
                  <BreadcrumbPage className="max-w-[200px] truncate">
                    {c.label}
                  </BreadcrumbPage>
                )}
              </BreadcrumbItem>
            </React.Fragment>
          ))}
        </BreadcrumbList>
      </Breadcrumb>

      {/* Page-published header actions live here, just left of search. */}
      <ChromeActionsSlot className="flex items-center gap-2 empty:hidden" />
      <AppSearch />
    </header>
  )
}
