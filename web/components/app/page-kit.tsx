import type { ReactNode } from "react"

import { cn } from "@/lib/utils"

// Shared building blocks for the application surfaces. Server-safe (no hooks) so
// any page can compose them.

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
