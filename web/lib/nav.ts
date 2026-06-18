import type { ElementType } from "react"
import {
  RiBarChartLine,
  RiBook2Line,
  RiFile2Line,
  RiFileList3Line,
  RiFileSearchLine,
  RiLayoutGrid2Line,
  RiNodeTree,
  RiSettings3Line,
  RiShieldKeyholeLine,
  RiSparkling2Line,
  RiTeamLine,
} from "@remixicon/react"

// The single source of truth for the app's primary navigation. Every nav surface
// — the spine, "/" search, the breadcrumb — derives from this, so labels and
// routes never drift.
export type NavGroup = "knowledge" | "manage" | "system"

export type NavItem = {
  key: string // first path segment
  label: string
  href: string
  group: NavGroup
  icon: ElementType // line variant (search, breadcrumb)
}

export const NAV: NavItem[] = [
  { key: "ask", label: "Ask", href: "/ask", group: "knowledge", icon: RiSparkling2Line }, // prettier-ignore
  { key: "book", label: "Book", href: "/book", group: "knowledge", icon: RiBook2Line }, // prettier-ignore
  { key: "sources", label: "Sources", href: "/sources", group: "manage", icon: RiLayoutGrid2Line }, // prettier-ignore
  { key: "files", label: "Files", href: "/files", group: "manage", icon: RiFile2Line }, // prettier-ignore
  { key: "entities", label: "Entities", href: "/entities", group: "manage", icon: RiNodeTree }, // prettier-ignore
  { key: "review", label: "Review", href: "/review", group: "manage", icon: RiFileSearchLine }, // prettier-ignore
  { key: "members", label: "Members", href: "/members", group: "manage", icon: RiTeamLine }, // prettier-ignore
  { key: "access", label: "Access", href: "/access", group: "manage", icon: RiShieldKeyholeLine }, // prettier-ignore
  { key: "audit", label: "Audit log", href: "/audit", group: "manage", icon: RiFileList3Line }, // prettier-ignore
  { key: "usage", label: "Usage", href: "/usage", group: "manage", icon: RiBarChartLine }, // prettier-ignore
  { key: "settings", label: "Settings", href: "/settings", group: "system", icon: RiSettings3Line }, // prettier-ignore
]

export const navByKey: Record<string, NavItem> = Object.fromEntries(
  NAV.map((n) => [n.key, n])
)
