"use client"

import * as React from "react"
import Link from "next/link"
import { usePathname } from "next/navigation"
import {
  RiBarChartFill,
  RiBook2Fill,
  RiFileList3Fill,
  RiFileSearchFill,
  RiLayoutGrid2Fill,
  RiLogoutBoxRLine,
  RiNodeTree,
  RiNotification3Fill,
  RiSettings3Fill,
  RiShieldKeyholeFill,
  RiSidebarFoldLine,
  RiSidebarUnfoldLine,
  RiSparkling2Fill,
  RiTeamFill,
  RiUser6Fill,
} from "@remixicon/react"

import { Logo } from "@/components/brand/logo"
import { useSession } from "@/lib/session"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { NAV, type NavItem } from "@/lib/nav"
import { cn } from "@/lib/utils"

// ── expand/collapse state ────────────────────────────────────────────────
// Same mechanics as shadcn's sidebar: a context with an expanded flag, a
// ⌘/Ctrl-B shortcut, and cookie persistence so the choice survives reloads.
const SPINE_COOKIE = "spine_expanded"
const SPINE_MAX_AGE = 60 * 60 * 24 * 30

type SpineContextValue = {
  expanded: boolean
  toggle: () => void
  setExpanded: (v: boolean) => void
}

const SpineContext = React.createContext<SpineContextValue | null>(null)

export function useSpine() {
  const ctx = React.useContext(SpineContext)
  if (!ctx) throw new Error("useSpine must be used within a SpineProvider")
  return ctx
}

export function SpineProvider({ children }: { children: React.ReactNode }) {
  const [expanded, setExpandedState] = React.useState(false)

  // Restore the persisted choice after mount (avoids a hydration mismatch).
  React.useEffect(() => {
    const m = document.cookie.match(/(?:^|; )spine_expanded=([^;]+)/)
    // eslint-disable-next-line react-hooks/set-state-in-effect -- browser-only cookie restore; reading it in an effect is the SSR-safe path
    if (m) setExpandedState(m[1] === "true")
  }, [])

  const setExpanded = React.useCallback((v: boolean) => {
    setExpandedState(v)
    document.cookie = `${SPINE_COOKIE}=${v}; path=/; max-age=${SPINE_MAX_AGE}`
  }, [])

  const toggle = React.useCallback(
    () => setExpanded(!expanded),
    [expanded, setExpanded]
  )

  React.useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "b") {
        e.preventDefault()
        toggle()
      }
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [toggle])

  const value = React.useMemo(
    () => ({ expanded, toggle, setExpanded }),
    [expanded, toggle, setExpanded]
  )

  return <SpineContext.Provider value={value}>{children}</SpineContext.Provider>
}

// ── the spine ────────────────────────────────────────────────────────────
// Labels and routes come from the shared registry (lib/nav); the spine renders
// the filled icon variants, mapped by key.
const FILL: Record<string, React.ElementType> = {
  ask: RiSparkling2Fill,
  book: RiBook2Fill,
  sources: RiLayoutGrid2Fill,
  entities: RiNodeTree,
  review: RiFileSearchFill,
  members: RiTeamFill,
  access: RiShieldKeyholeFill,
  audit: RiFileList3Fill,
  usage: RiBarChartFill,
  settings: RiSettings3Fill,
}

const KNOWLEDGE = NAV.filter((n) => n.group === "knowledge")
const MANAGE = NAV.filter((n) => n.group === "manage")
const SETTINGS = NAV.find((n) => n.key === "settings")!

export function AppSpine() {
  const pathname = usePathname()
  const { expanded } = useSpine()
  const isActive = (href: string) =>
    href === "/book" ? pathname.startsWith("/book") : pathname.startsWith(href)

  return (
    <TooltipProvider delayDuration={300}>
      <nav
        data-state={expanded ? "expanded" : "collapsed"}
        className={cn(
          "group/spine flex shrink-0 flex-col overflow-y-auto border-r border-spine-border bg-spine py-2 transition-[width] duration-200 ease-linear motion-reduce:transition-none",
          expanded ? "w-60" : "w-14"
        )}
      >
        <SpineHeader />

        <Divider />

        {KNOWLEDGE.map((item) => (
          <SpineLink key={item.href} item={item} active={isActive(item.href)} />
        ))}

        <SectionBreak label="Manage" />
        {MANAGE.map((item) => (
          <SpineLink key={item.href} item={item} active={isActive(item.href)} />
        ))}

        <div className="flex-1" />

        <Notifications />
        <SpineLink item={SETTINGS} active={isActive(SETTINGS.href)} />
        <ProfileMenu />
      </nav>
    </TooltipProvider>
  )
}

// Full-bleed rows (no nav padding, no gap, no rounding) so active/hover reads as
// a full-width block and clickable areas are contiguous. Every leading glyph
// sits in a 28px slot with CONSTANT row padding, so it's centered when collapsed
// (14 + 28 + 14 = 56) and stays at the exact same x when expanded — no movement.
const rowBase =
  "flex h-9 w-full items-center gap-2.5 px-3.5 text-[13px] font-medium transition-colors"

// Fixed-width slot for the leading icon/badge so collapsed centering is uniform.
function Glyph({ children }: { children: React.ReactNode }) {
  return (
    <span className="flex size-7 shrink-0 items-center justify-center">
      {children}
    </span>
  )
}

function Label({ children }: { children: React.ReactNode }) {
  return (
    <span className="min-w-0 flex-1 truncate text-left group-data-[state=collapsed]/spine:hidden">
      {children}
    </span>
  )
}

function SpineLink({ item, active }: { item: NavItem; active: boolean }) {
  const Icon = FILL[item.key]
  const { expanded } = useSpine()
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Link
          href={item.href}
          aria-current={active ? "page" : undefined}
          className={cn(
            rowBase,
            active
              ? "bg-spine-accent text-spine-foreground"
              : "text-spine-muted hover:bg-spine-accent/60 hover:text-spine-foreground"
          )}
        >
          <Glyph>
            <Icon className="size-4.5" />
          </Glyph>
          <Label>{item.label}</Label>
        </Link>
      </TooltipTrigger>
      <TooltipContent side="right" hidden={expanded}>
        {item.label}
      </TooltipContent>
    </Tooltip>
  )
}

function Divider() {
  return <div className="my-1 h-px w-full bg-spine-border" />
}

// A section divider with a fixed height in BOTH states — the label when open,
// a centered hairline when collapsed — so items below it never shift vertically
// as the sidebar toggles.
function SectionBreak({ label }: { label: string }) {
  return (
    <div className="flex h-7 items-center">
      <span className="px-3.5 text-[10px] font-medium tracking-wide text-spine-muted/70 uppercase group-data-[state=collapsed]/spine:hidden">
        {label}
      </span>
      <span className="hidden h-px w-full bg-spine-border group-data-[state=collapsed]/spine:block" />
    </div>
  )
}

// Top of the spine. Collapsed: a centered unfold glyph that expands the spine.
// Expanded: the logo with a right-aligned fold button that collapses it — the
// toggle lives next to what it controls, so the context bar carries none.
function SpineHeader() {
  const { expanded, toggle } = useSpine()

  if (!expanded) {
    return (
      <button
        type="button"
        onClick={toggle}
        aria-label="Expand sidebar"
        className={cn(
          rowBase,
          "text-spine-muted transition-colors hover:bg-spine-accent hover:text-spine-foreground focus-visible:ring-2 focus-visible:ring-primary focus-visible:outline-none"
        )}
      >
        <Glyph>
          <RiSidebarUnfoldLine className="size-4.5" />
        </Glyph>
      </button>
    )
  }

  return (
    <div className={cn(rowBase, "text-spine-foreground")}>
      <Logo size="sm" className="px-0.5" />
      <div className="min-w-0 flex-1" />
      <button
        type="button"
        onClick={toggle}
        aria-label="Collapse sidebar"
        className="flex size-7 shrink-0 items-center justify-center rounded-md text-spine-muted transition-colors hover:bg-spine-accent hover:text-spine-foreground focus-visible:ring-2 focus-visible:ring-primary focus-visible:outline-none"
      >
        <RiSidebarFoldLine className="size-4.5" />
      </button>
    </div>
  )
}

function Notifications() {
  const { expanded } = useSpine()
  return (
    <Tooltip>
      <DropdownMenu>
        <TooltipTrigger asChild>
          <DropdownMenuTrigger
            className={cn(
              rowBase,
              "text-spine-muted hover:bg-spine-accent/60 hover:text-spine-foreground focus-visible:ring-2 focus-visible:ring-primary focus-visible:outline-none"
            )}
          >
            <Glyph>
              <RiNotification3Fill className="size-4.5" />
            </Glyph>
            <Label>Notifications</Label>
          </DropdownMenuTrigger>
        </TooltipTrigger>
        <DropdownMenuContent align="end" side="right" className="w-64">
          <DropdownMenuLabel className="text-[12px] font-medium tracking-wide text-muted-foreground uppercase">
            Notifications
          </DropdownMenuLabel>
          <p className="px-2 py-6 text-center text-[13px] text-muted-foreground">
            Notifications aren&rsquo;t built yet.
          </p>
        </DropdownMenuContent>
      </DropdownMenu>
      <TooltipContent side="right" hidden={expanded}>
        Notifications
      </TooltipContent>
    </Tooltip>
  )
}

function ProfileMenu() {
  // Identity and workspace both come from the server session — never derived from
  // client-side OIDC state (design-doc 0005). One shared query, deduplicated with
  // every other consumer (design-doc 0006).
  const { data } = useSession()

  const name = data?.user?.name || data?.user?.email || ""
  const email = data?.user?.email || ""
  const orgName = data?.tenant?.name || null
  const initial = name ? name.trim().charAt(0).toUpperCase() : ""

  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger
        className={cn(
          rowBase,
          "mt-0.5 text-spine-foreground hover:bg-spine-accent focus-visible:ring-2 focus-visible:ring-primary focus-visible:outline-none"
        )}
      >
        <span className="flex size-7 shrink-0 items-center justify-center rounded-full bg-brand-subtle text-[12px] font-semibold text-brand-subtle-foreground">
          {initial || <RiUser6Fill className="size-4" />}
        </span>
        <span className="min-w-0 flex-1 truncate text-left group-data-[state=collapsed]/spine:hidden">
          {name}
        </span>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" side="right" className="w-60">
        <div className="px-2 py-1.5">
          {name && (
            <p className="text-[13px] leading-tight font-medium text-foreground">
              {name}
            </p>
          )}
          {email && (
            <p className="text-[12px] text-muted-foreground">{email}</p>
          )}
        </div>
        {orgName && (
          <>
            <DropdownMenuSeparator />
            <div className="px-2 py-1.5">
              <p className="text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
                Workspace
              </p>
              <p className="mt-0.5 truncate text-[13px] text-foreground">
                {orgName}
              </p>
            </div>
          </>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild className="gap-2 text-[13px]">
          <Link href="/account">
            <RiSettings3Fill className="size-3.5 text-muted-foreground" />
            Account settings
          </Link>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild className="gap-2 text-[13px]">
          <Link href="/signed-out">
            <RiLogoutBoxRLine className="size-3.5 text-muted-foreground" />
            Sign out
          </Link>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
