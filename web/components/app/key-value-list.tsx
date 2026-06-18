import { type ReactNode } from "react"

import { cn } from "@/lib/utils"

export type KeyValueItem = {
  label: string
  value: ReactNode
}

// KeyValueList renders metadata as labelled rows — fixed-width muted label,
// foreground value, quiet dividers. The one way metadata reads across the app
// (connector details, settings) so it never drifts. Apex: restraint, weight over
// colour.
export function KeyValueList({
  items,
  className,
}: {
  items: KeyValueItem[]
  className?: string
}) {
  return (
    <dl className={cn("text-[13px]", className)}>
      {items.map((item) => (
        <div
          key={item.label}
          className="flex gap-3 border-b border-border/60 py-2 last:border-0"
        >
          <dt className="w-32 shrink-0 text-muted-foreground">{item.label}</dt>
          <dd className="min-w-0 flex-1 text-foreground">{item.value}</dd>
        </div>
      ))}
    </dl>
  )
}
