"use client"

import Link from "next/link"
import {
  RiAddLine,
  RiArrowDownSLine,
  RiCheckLine,
  RiLoginBoxLine,
  RiLogoutBoxRLine,
  RiSettings3Line,
} from "@remixicon/react"

import { Logo } from "@/components/brand/logo"
import { StylePicker } from "@/components/brand/style-picker"
import { mockOrg, mockUser, mockTenants } from "@/lib/mock-data"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

// Persistent app chrome shown across onboarding (and, later, the app):
// workspace switcher on the left, profile on the right.
export function OnboardingTopBar() {
  return (
    <header className="relative z-20 flex shrink-0 items-center justify-between gap-3 bg-background px-6 py-3">
      <div className="flex min-w-0 items-center gap-2.5">
        <Logo variant="mark" size="sm" />
        <span aria-hidden className="text-border">
          /
        </span>
        <WorkspaceSwitcher />
      </div>

      <div className="flex items-center gap-3">
        {/* DEMO ONLY — theming control; not real auth/app chrome. */}
        <StylePicker />
        <ProfileMenu />
      </div>
    </header>
  )
}

function Badge({ children }: { children: React.ReactNode }) {
  return (
    <span className="flex size-5 shrink-0 items-center justify-center rounded bg-brand-subtle text-[12px] font-semibold text-brand-subtle-foreground">
      {children}
    </span>
  )
}

function WorkspaceSwitcher() {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger className="group inline-flex min-w-0 items-center gap-2 rounded-md px-1.5 py-1 text-[13px] text-foreground transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
        <Badge>{mockOrg.name[0]}</Badge>
        <span className="truncate font-medium">{mockOrg.name}</span>
        <RiArrowDownSLine className="size-3.5 shrink-0 text-muted-foreground transition-transform group-data-[state=open]:rotate-180" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-64">
        <DropdownMenuLabel className="text-[12px] font-medium uppercase tracking-wide text-muted-foreground">
          Workspaces
        </DropdownMenuLabel>
        {mockTenants.map((t) => (
          <DropdownMenuItem key={t.slug} className="flex items-center gap-2 text-[13px]">
            <Badge>{t.name[0]}</Badge>
            <span className="min-w-0 flex-1 truncate">{t.name}</span>
            <span className="text-[12px] text-muted-foreground">{t.role}</span>
            {t.slug === mockOrg.slug && (
              <RiCheckLine className="size-3.5 shrink-0 text-foreground" />
            )}
          </DropdownMenuItem>
        ))}
        <DropdownMenuSeparator />
        <DropdownMenuItem className="gap-2 text-[13px]">
          <RiAddLine className="size-3.5 text-muted-foreground" />
          Create workspace
        </DropdownMenuItem>
        <DropdownMenuItem className="gap-2 text-[13px]">
          <RiLoginBoxLine className="size-3.5 text-muted-foreground" />
          Join workspace
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function ProfileMenu() {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger className="inline-flex size-7 items-center justify-center rounded-full bg-brand-subtle text-[12px] font-semibold text-brand-subtle-foreground transition-shadow hover:shadow-[0_0_0_4px_rgb(0_0_0_/_0.04)] focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
        {mockUser.initial}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-60">
        <div className="px-2 py-1.5">
          <p className="text-[13px] font-medium leading-tight text-foreground">
            {mockUser.name}
          </p>
          <p className="text-[12px] text-muted-foreground">{mockUser.email}</p>
        </div>
        <DropdownMenuSeparator />
        <DropdownMenuItem className="gap-2 text-[13px]">
          <RiSettings3Line className="size-3.5 text-muted-foreground" />
          Settings
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
