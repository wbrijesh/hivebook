import Link from "next/link"
import { RiArrowRightSLine } from "@remixicon/react"

import { Logo } from "@/components/brand/logo"
import { SiteFooter } from "@/components/brand/site-footer"
import { StylePicker } from "@/components/brand/style-picker"

type Page = { href: string; label: string; detail: string }
type Group = { title: string; description: string; pages: Page[] }

const groups: Group[] = [
  {
    title: "Application",
    description: "The signed-in app — dense, engineered chrome. Book reading is real; admin surfaces are navigable stubs for now.",
    pages: [
      { href: "/book", label: "Book home", detail: "Chapters + recently updated, with the cold-start state." },
      { href: "/book/s-eu-vat", label: "Entry — sub-topic", detail: "Summary with inline citations → source-span drawer." },
      { href: "/book/t-refunds", label: "Entry — topic", detail: "Summary + child sub-topics." },
      { href: "/book/ch-cust", label: "Entry — building", detail: "Cold-start empty chapter." },
      { href: "/ask", label: "Ask", detail: "Cited answers (stub). ⌘K also asks." },
      { href: "/sources", label: "Sources", detail: "Connector health (stub)." },
      { href: "/members", label: "Members", detail: "Roles + invites (stub)." },
      { href: "/access", label: "Access", detail: "Chapter ACL + templates (stub)." },
      { href: "/entities", label: "Entities", detail: "Entity store (stub)." },
      { href: "/entities/ambiguity", label: "Ambiguity queue", detail: "Resolve ambiguous entities (stub)." },
      { href: "/review", label: "Review", detail: "Structural proposals + unfiled (stub)." },
      { href: "/audit", label: "Audit log", detail: "Read/write events (stub)." },
      { href: "/usage", label: "Usage", detail: "Metering (stub)." },
      { href: "/settings", label: "Settings", detail: "Workspace, security, data (stub)." },
      { href: "/account", label: "Account", detail: "Personal settings (stub)." },
    ],
  },
  {
    title: "Authentication",
    description: "One email-first door: SSO or work email, then verify or password, and the dead-end states.",
    pages: [
      { href: "/sign-in", label: "Sign in", detail: "Front door — SSO providers, or work email. Known email → password, new → verify." },
      { href: "/enter-password", label: "Enter password", detail: "Returning user — password, with an email-me-a-code escape hatch." },
      { href: "/verify-email", label: "Verify email (OTP)", detail: "6-digit code; routes by ?flow (signup · login · reset)." },
      { href: "/set-password", label: "Set password", detail: "Create (default) or reset (?mode=reset) — strength rules + confirm." },
      { href: "/forgot-password", label: "Forgot password", detail: "Request a code by email, then set a new password." },
      { href: "/sso/callback", label: "SSO callback", detail: "Animated multi-step loading state." },
      { href: "/signed-out", label: "Signed out", detail: "Post sign-out confirmation." },
      { href: "/session-expired", label: "Session expired", detail: "Timed-out session message." },
      { href: "/error", label: "Auth error", detail: "Generic failure with reference ID." },
    ],
  },
  {
    title: "Onboarding",
    description: "Short, focused walkthrough — one question per screen, no skips.",
    pages: [
      { href: "/onboarding/organization", label: "Step 1 — Organization", detail: "Name (pre-filled) + company size." },
      { href: "/onboarding/use-case", label: "Step 2 — What it's for", detail: "Operational areas → starting chapters." },
      { href: "/onboarding/region", label: "Step 3 — Region", detail: "Storage cluster, recommended pre-selected." },
      { href: "/onboarding/complete", label: "Complete", detail: "Ready + next steps (connect a source in the app)." },
    ],
  },
]

export default function PrototypeIndex() {
  return (
    <div className="relative flex min-h-svh flex-col bg-background">
      <header className="flex items-center justify-between px-6 py-5">
        <Logo />
        <div className="flex items-center gap-3">
          <span className="rounded-md border border-border bg-background px-2 py-1 text-[11px] font-medium tracking-wide text-muted-foreground">
            PROTOTYPE INDEX
          </span>
          <StylePicker />
        </div>
      </header>

      <main className="flex-1 px-6 py-10">
        <div className="mx-auto max-w-3xl">
          <div className="mb-10">
            <h1 className="text-[28px] font-semibold tracking-tight text-foreground">
              Trenches frontend prototype
            </h1>
            <p className="mt-2 max-w-xl text-[14px] leading-relaxed text-muted-foreground">
              A click-through of every page we&apos;ve built so far. We&apos;re
              building the surface before the backend to discover, by usage,
              what the system needs to do. This index is a dev convenience and
              won&apos;t exist in production.
            </p>
          </div>

          <div className="space-y-10">
            {groups.map((g) => (
              <section key={g.title}>
                <header className="mb-3">
                  <h2 className="text-[16px] font-semibold tracking-tight text-foreground">
                    {g.title}
                  </h2>
                  <p className="text-[12.5px] text-muted-foreground">
                    {g.description}
                  </p>
                </header>
                <ul className="divide-y divide-border rounded-xl border border-border bg-card">
                  {g.pages.map((p) => (
                    <li key={p.href}>
                      <Link
                        href={p.href}
                        className="group flex items-center gap-3 px-4 py-3 transition-colors hover:bg-muted/50"
                      >
                        <code className="rounded-md bg-muted px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground">
                          {p.href}
                        </code>
                        <span className="flex-1 truncate">
                          <span className="text-[13.5px] font-medium text-foreground">
                            {p.label}
                          </span>
                          <span className="ml-2 text-[12px] text-muted-foreground">
                            {p.detail}
                          </span>
                        </span>
                        <RiArrowRightSLine className="size-4 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
                      </Link>
                    </li>
                  ))}
                </ul>
              </section>
            ))}
          </div>
        </div>
      </main>

      <SiteFooter />
    </div>
  )
}
