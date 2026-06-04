import * as React from "react"
import { RiCheckLine } from "@remixicon/react"

import { cn } from "@/lib/utils"

// Label + control pair. One field per row.
export function Field({
  label,
  htmlFor,
  hint,
  trailing,
  children,
}: {
  label?: React.ReactNode
  htmlFor?: string
  hint?: React.ReactNode
  trailing?: React.ReactNode
  children: React.ReactNode
}) {
  return (
    <div className="space-y-1.5">
      {(label || trailing) && (
        <div className="flex items-center justify-between">
          {label && (
            <label
              htmlFor={htmlFor}
              className="text-[13px] font-medium text-foreground"
            >
              {label}
            </label>
          )}
          {trailing}
        </div>
      )}
      {children}
      {hint && (
        <p className="text-[12px] leading-relaxed text-muted-foreground">
          {hint}
        </p>
      )}
    </div>
  )
}

export type Option = { id: string; label: string; detail?: string }

// The one selection pattern: bordered cells, accent border + check when chosen.
// Restrained and systemic — not playful pills. Works for single- and
// multi-select; the parent owns state.
export function OptionGrid({
  options,
  selected,
  onSelect,
  columns = 1,
}: {
  options: Option[]
  selected: (id: string) => boolean
  onSelect: (id: string) => void
  columns?: 1 | 2 | 3
}) {
  return (
    <div
      className={cn(
        "grid gap-2",
        columns === 2 && "sm:grid-cols-2",
        columns === 3 && "sm:grid-cols-3"
      )}
    >
      {options.map((o) => {
        const on = selected(o.id)
        return (
          <button
            key={o.id}
            type="button"
            aria-pressed={on}
            onClick={() => onSelect(o.id)}
            className={cn(
              // border-2 on every cell so the selected accent edge is thicker
              // without the content shifting when selection changes.
              "flex items-center gap-2.5 rounded-lg border-2 px-3.5 py-2.5 text-left transition-colors focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
              on
                ? "border-primary bg-card"
                : "border-border bg-card hover:bg-muted"
            )}
          >
            <span className="min-w-0 flex-1">
              <span className="block text-[14px] text-foreground">
                {o.label}
              </span>
              {o.detail && (
                <span className="mt-0.5 block text-[12px] text-muted-foreground">
                  {o.detail}
                </span>
              )}
            </span>
            <RiCheckLine
              className={cn(
                "size-4 shrink-0",
                on ? "text-primary" : "text-transparent"
              )}
            />
          </button>
        )
      })}
    </div>
  )
}
