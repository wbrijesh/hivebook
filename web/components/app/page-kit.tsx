import type { ReactNode } from "react"

import { cn } from "@/lib/utils"

// Shared building blocks for the application surfaces. Server-safe (no hooks) so
// any page can compose them.

// PageShell is full-width — no centered max-width container. The work area uses
// the whole pane (tables/lists earn the space on big displays); content that needs
// a comfortable measure (forms, prose) constrains itself, not the page. Padding is
// responsive so it stays sensible on small screens.
export function PageShell({
  children,
  className,
}: {
  children: ReactNode
  className?: string
}) {
  return (
    <div className="h-full overflow-y-auto">
      <div className={cn("w-full px-4 py-6 sm:px-6", className)}>
        {children}
      </div>
    </div>
  )
}

// Page-level actions don't live here — they go in the application chrome (the
// context bar, left of search) via ChromeActions, so the page body stays content
// only. PageHeading is title + description.
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
