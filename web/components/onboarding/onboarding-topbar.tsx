"use client"

import * as React from "react"
import Link from "next/link"
import { RiLogoutBoxRLine, RiUser6Fill } from "@remixicon/react"

import { Logo } from "@/components/brand/logo"
import { fetchMe } from "@/lib/tenant"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

// Minimal chrome for onboarding: the Hivebook mark and the signed-in user's
// account menu. No workspace name or switcher — the org is being created on this
// very screen, and there is no multi-workspace feature.
export function OnboardingTopBar() {
  return (
    <header className="relative z-20 flex shrink-0 items-center justify-between gap-3 bg-background px-6 py-3">
      <Logo variant="full" size="sm" />
      <ProfileMenu />
    </header>
  )
}

type Profile = { name: string; email: string }

function ProfileMenu() {
  const [p, setP] = React.useState<Profile | null>(null)

  // Identity comes from the server (/api/me), not client OIDC state (design-doc 0005).
  React.useEffect(() => {
    fetchMe()
      .then((s) => {
        if (s) setP({ name: s.user.name, email: s.user.email })
      })
      .catch(() => {})
  }, [])

  return (
    <DropdownMenu>
      <DropdownMenuTrigger className="inline-flex size-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none">
        <RiUser6Fill className="size-5" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-60">
        {(p?.name || p?.email) && (
          <>
            <div className="px-2 py-1.5">
              {p?.name && (
                <p className="text-[13px] leading-tight font-medium text-foreground">
                  {p.name}
                </p>
              )}
              {p?.email && (
                <p className="text-[12px] text-muted-foreground">{p.email}</p>
              )}
            </div>
            <DropdownMenuSeparator />
          </>
        )}
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
