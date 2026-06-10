import { RiBrain4Fill } from "@remixicon/react"

import { cn } from "@/lib/utils"

type LogoProps = {
  className?: string
  variant?: "full" | "mark"
  size?: "sm" | "md" | "lg"
}

const sizes = {
  sm: { mark: "size-5", text: "text-sm" },
  md: { mark: "size-6", text: "text-[16px]" },
  lg: { mark: "size-8", text: "text-lg" },
}

export function Logo({ className, variant = "full", size = "md" }: LogoProps) {
  const s = sizes[size]
  return (
    <span className={cn("inline-flex items-center gap-2", className)}>
      <RiBrain4Fill className={s.mark} aria-hidden />
      {variant === "full" && (
        <span className={cn("font-semibold tracking-tight", s.text)}>
          Hivebook
        </span>
      )}
    </span>
  )
}
