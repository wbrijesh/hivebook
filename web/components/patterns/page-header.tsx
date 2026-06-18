import type { ReactNode } from "react"
import Link from "next/link"
import { RiArrowLeftLine } from "@remixicon/react"

import { Button } from "@/components/ui/button"

// PageHeader is the shared page-header pattern used across detail and list pages:
// an optional back link, an optional leading icon, a title + optional subtitle, and
// an optional right-aligned `actions` element (usually a fragment of buttons — the
// page owns which buttons, the header only exposes the slot). Every header keeps the
// same shape; pass only what a given page needs. Server-safe (no hooks).
export function PageHeader({
  back,
  backLabel = "Back",
  icon,
  title,
  subtitle,
  actions,
}: {
  // A real href to navigate back to (renders the back button when set).
  back?: string
  backLabel?: string
  icon?: ReactNode
  title: ReactNode
  subtitle?: ReactNode
  actions?: ReactNode
}) {
  return (
    <div className="flex items-center justify-between gap-3">
      <div className="flex min-w-0 items-center gap-3">
        {back && (
          <Button
            asChild
            variant="outline"
            size="icon-sm"
            aria-label={backLabel}
          >
            <Link href={back}>
              <RiArrowLeftLine />
            </Link>
          </Button>
        )}
        <div className="flex min-w-0 items-center gap-2">
          {icon}
          <div className="min-w-0 leading-tight">
            <h1 className="truncate text-[15px] font-semibold tracking-tight text-foreground">
              {title}
            </h1>
            {subtitle && (
              <p className="truncate text-[12px] text-muted-foreground">
                {subtitle}
              </p>
            )}
          </div>
        </div>
      </div>
      {actions && (
        <div className="flex shrink-0 items-center gap-2">{actions}</div>
      )}
    </div>
  )
}
