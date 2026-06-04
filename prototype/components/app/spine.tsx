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
  RiSearchLine,
  RiSettings3Fill,
  RiShieldKeyholeFill,
  RiSidebarFoldLine,
  RiSidebarUnfoldLine,
  RiSparkling2Fill,
  RiTeamFill,
} from "@remixicon/react"

import { useCommandMenu } from "@/components/app/command-menu"
import { Logo } from "@/components/brand/logo"
import { Kbd } from "@/components/ui/kbd"
import { mockUser, mockTenants } from "@/lib/mock-data"
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
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
    if (m) setExpandedState(m[1] === "true")
  }, [])

  const setExpanded = React.useCallback((v: boolean) => {
    setExpandedState(v)
    document.cookie = `${SPINE_COOKIE}=${v}; path=/; max-age=${SPINE_MAX_AGE}`
  }, [])

  const toggle = React.useCallback(() => setExpanded(!expanded), [expanded, setExpanded])

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
type Item = { label: string; href: string; icon: React.ElementType }

const KNOWLEDGE: Item[] = [
  { label: "Ask", href: "/ask", icon: RiSparkling2Fill },
  { label: "Book", href: "/book", icon: RiBook2Fill },
]

const MANAGE: Item[] = [
  { label: "Sources", href: "/sources", icon: RiLayoutGrid2Fill },
  { label: "Entities", href: "/entities", icon: RiNodeTree },
  { label: "Review", href: "/review", icon: RiFileSearchFill },
  { label: "Members", href: "/members", icon: RiTeamFill },
  { label: "Access", href: "/access", icon: RiShieldKeyholeFill },
  { label: "Audit", href: "/audit", icon: RiFileList3Fill },
  { label: "Usage", href: "/usage", icon: RiBarChartFill },
]

export function AppSpine() {
  const pathname = usePathname()
  const cmd = useCommandMenu()
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

        <SpineAction
          label="Search"
          kbd="⌘K"
          icon={RiSearchLine}
          onClick={cmd.open}
          className="border border-spine-border bg-spine-accent/40 text-spine-muted hover:bg-spine-accent hover:text-spine-foreground"
        />
        {KNOWLEDGE.map((item) => (
          <SpineLink key={item.href} item={item} active={isActive(item.href)} />
        ))}

        <SectionBreak label="Manage" />
        {MANAGE.map((item) => (
          <SpineLink key={item.href} item={item} active={isActive(item.href)} />
        ))}

        <div className="flex-1" />

        <Notifications />
        <SpineLink
          item={{ label: "Settings", href: "/settings", icon: RiSettings3Fill }}
          active={isActive("/settings")}
        />
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
    <span className="flex size-7 shrink-0 items-center justify-center">{children}</span>
  )
}

function Label({ children }: { children: React.ReactNode }) {
  return (
    <span className="min-w-0 flex-1 truncate text-left group-data-[state=collapsed]/spine:hidden">
      {children}
    </span>
  )
}

function SpineLink({ item, active }: { item: Item; active: boolean }) {
  const Icon = item.icon
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

function SpineAction({
  label,
  kbd,
  icon: Icon,
  onClick,
  className,
}: {
  label: string
  kbd?: string
  icon: React.ElementType
  onClick: () => void
  className?: string
}) {
  const { expanded } = useSpine()
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          onClick={onClick}
          className={cn(
            rowBase,
            "text-spine-muted hover:bg-spine-accent/60 hover:text-spine-foreground",
            className
          )}
        >
          <Glyph>
            <Icon className="size-4.5" />
          </Glyph>
          <Label>{label}</Label>
          {kbd && (
            <Kbd className="shrink-0 border border-spine-muted/40 bg-transparent text-spine-muted group-data-[state=collapsed]/spine:hidden">
              {kbd}
            </Kbd>
          )}
        </button>
      </TooltipTrigger>
      <TooltipContent side="right" className="gap-2" hidden={expanded}>
        {label}
        {kbd && <Kbd>{kbd}</Kbd>}
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
      <span className="px-3.5 text-[10px] font-medium uppercase tracking-wide text-spine-muted/70 group-data-[state=collapsed]/spine:hidden">
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
          "text-spine-muted transition-colors hover:bg-spine-accent hover:text-spine-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
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
        className="flex size-7 shrink-0 items-center justify-center rounded-md text-spine-muted transition-colors hover:bg-spine-accent hover:text-spine-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
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
            className={cn(rowBase, "text-spine-muted hover:bg-spine-accent/60 hover:text-spine-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary")}
          >
            <Glyph>
              <RiNotification3Fill className="size-4.5" />
            </Glyph>
            <Label>Notifications</Label>
          </DropdownMenuTrigger>
        </TooltipTrigger>
        <DropdownMenuContent align="end" side="right" className="w-64">
          <DropdownMenuLabel className="text-[12px] font-medium uppercase tracking-wide text-muted-foreground">
            Notifications
          </DropdownMenuLabel>
          <p className="px-2 py-6 text-center text-[13px] text-muted-foreground">
            You're all caught up.
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
  const [org, setOrg] = React.useState(mockTenants[0].slug)
  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger
        className={cn(rowBase, "mt-0.5 text-spine-foreground hover:bg-spine-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary")}
      >
        <span className="flex size-7 shrink-0 items-center justify-center rounded-full bg-brand-subtle text-[12px] font-semibold text-brand-subtle-foreground">
          {mockUser.initial}
        </span>
        <span className="min-w-0 flex-1 truncate text-left group-data-[state=collapsed]/spine:hidden">
          {mockUser.name}
        </span>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        side="right"
        className="w-60"
        // Keep the menu open while the nested org select (a separate portal) is used.
        onInteractOutside={(e) => {
          if ((e.target as Element | null)?.closest?.('[data-slot="select-content"]')) {
            e.preventDefault()
          }
        }}
      >
        <div className="px-2 py-1.5">
          <p className="text-[13px] font-medium leading-tight text-foreground">
            {mockUser.name}
          </p>
          <p className="text-[12px] text-muted-foreground">{mockUser.email}</p>
        </div>
        <DropdownMenuSeparator />
        <div className="px-1.5 py-1">
          <p className="mb-1 text-[12px] font-medium text-muted-foreground">
            Organization
          </p>
          <Select value={org} onValueChange={setOrg}>
            <SelectTrigger size="sm" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {mockTenants.map((t) => (
                <SelectItem key={t.slug} value={t.slug}>
                  {t.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
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
