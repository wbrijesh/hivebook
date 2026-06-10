import type { ReactNode } from "react"
import Link from "next/link"

import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"

// Shared building blocks for the dense application surfaces. Server-safe (no
// hooks) so any page can compose them; interactive pages add their own client
// pieces on top. Density follows the Apex "system surfaces" rules: 11px column
// heads, 12–13px content, tabular figures, quiet rows.

export function PageShell({
  children,
  width = "wide",
  className,
}: {
  children: ReactNode
  width?: "wide" | "reading"
  className?: string
}) {
  return (
    <div className="h-full overflow-y-auto">
      <div
        className={cn(
          "mx-auto px-8 py-7",
          width === "wide" ? "max-w-[1100px]" : "max-w-[760px]",
          className
        )}
      >
        {children}
      </div>
    </div>
  )
}

export function PageHeading({
  title,
  description,
}: {
  title: string
  description?: string
}) {
  return (
    <div>
      <h1 className="text-[15px] font-semibold tracking-tight text-foreground">
        {title}
      </h1>
      {description && (
        <p className="mt-0.5 text-[13px] text-muted-foreground">
          {description}
        </p>
      )}
    </div>
  )
}

export function StatBar({
  items,
}: {
  items: { label: string; value: string; sub?: string }[]
}) {
  return (
    <div className="mt-4 flex flex-wrap divide-x divide-border overflow-hidden rounded-md border border-border bg-card">
      {items.map((s) => (
        <div key={s.label} className="min-w-[136px] flex-1 px-4 py-2.5">
          <div className="text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
            {s.label}
          </div>
          <div className="mt-0.5 text-[18px] leading-none font-semibold text-foreground tabular-nums">
            {s.value}
          </div>
          {s.sub && (
            <div className="mt-1 text-[12px] text-muted-foreground">
              {s.sub}
            </div>
          )}
        </div>
      ))}
    </div>
  )
}

export function Section({
  title,
  count,
  action,
  children,
}: {
  title: string
  count?: number
  action?: ReactNode
  children: ReactNode
}) {
  return (
    <section className="mt-7">
      <div className="mb-2 flex items-center gap-2">
        <h2 className="text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
          {title}
        </h2>
        {count !== undefined && (
          <span className="font-mono text-[11px] text-muted-foreground/70">
            {count}
          </span>
        )}
        {action && <div className="ml-auto">{action}</div>}
      </div>
      {children}
    </section>
  )
}

export function TableCard({ children }: { children: ReactNode }) {
  return (
    <div className="overflow-hidden rounded-md border border-border bg-card">
      {children}
    </div>
  )
}

export function HeadRow({ children }: { children: ReactNode }) {
  return (
    <div className="flex items-center gap-3 border-b border-border bg-muted/40 px-3.5 py-1.5">
      {children}
    </div>
  )
}

export function Th({
  children,
  className,
}: {
  children: ReactNode
  className?: string
}) {
  return (
    <div
      className={cn(
        "text-[11px] font-medium tracking-wide text-muted-foreground uppercase",
        className
      )}
    >
      {children}
    </div>
  )
}

export function Row({
  children,
  href,
}: {
  children: ReactNode
  href?: string
}) {
  const cls =
    "flex items-center gap-3 border-b border-border px-3.5 py-2 text-[13px] last:border-b-0"
  if (href) {
    return (
      <Link
        href={href}
        className={cn(cls, "transition-colors hover:bg-muted/50")}
      >
        {children}
      </Link>
    )
  }
  return <div className={cls}>{children}</div>
}

export function Td({
  children,
  className,
}: {
  children: ReactNode
  className?: string
}) {
  return (
    <div className={cn("flex min-w-0 items-center", className)}>{children}</div>
  )
}

export function Num({
  children,
  className,
}: {
  children: ReactNode
  className?: string
}) {
  return (
    <div
      className={cn(
        "shrink-0 text-right font-mono text-[12px] text-foreground tabular-nums",
        className
      )}
    >
      {children}
    </div>
  )
}

type Tone = "success" | "warning" | "destructive" | "info" | "muted"

const TONE_DOT: Record<Tone, string> = {
  success: "bg-success",
  warning: "bg-warning",
  destructive: "bg-destructive",
  info: "bg-primary",
  muted: "bg-muted-foreground/50",
}

// Map our semantic tones to the shadcn Badge variants. Tag is a thin wrapper so
// every status/metadata label routes through the one Badge component.
const TONE_VARIANT: Record<
  Tone,
  "brand" | "muted" | "success" | "warning" | "destructive"
> = {
  success: "success",
  warning: "warning",
  destructive: "destructive",
  info: "brand",
  muted: "muted",
}

export function Dot({ tone, pulse }: { tone: Tone; pulse?: boolean }) {
  return (
    <span
      className={cn(
        "size-1.5 shrink-0 rounded-full",
        TONE_DOT[tone],
        pulse && "animate-pulse"
      )}
    />
  )
}

export function Tag({
  children,
  tone = "muted",
}: {
  children: ReactNode
  tone?: Tone
}) {
  return <Badge variant={TONE_VARIANT[tone]}>{children}</Badge>
}

// A small letter tile for people (reuses the brand-subtle palette).
export function Avatar({ initial }: { initial: string }) {
  return (
    <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-brand-subtle text-[11px] font-semibold text-brand-subtle-foreground">
      {initial}
    </span>
  )
}

export function fmtCount(n: number): string {
  if (n >= 1_000_000)
    return `${(n / 1_000_000).toFixed(2).replace(/\.?0+$/, "")}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1).replace(/\.0$/, "")}k`
  return `${n}`
}
