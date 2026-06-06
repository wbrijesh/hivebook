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
    <span className={cn("inline-flex items-center gap-2 text-foreground", className)}>
      <LogoMark className={s.mark} />
      {variant === "full" && (
        <span className={cn("font-semibold tracking-tight", s.text)}>
          Hivebook
        </span>
      )}
    </span>
  )
}

function LogoMark({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
      className={cn("text-foreground", className)}
    >
      <rect x="3" y="4.5" width="18" height="3.2" rx="0.6" fill="currentColor" />
      <rect x="3" y="10.4" width="18" height="3.2" rx="0.6" fill="currentColor" opacity="0.55" />
      <rect x="3" y="16.3" width="18" height="3.2" rx="0.6" fill="currentColor" opacity="0.22" />
    </svg>
  )
}
