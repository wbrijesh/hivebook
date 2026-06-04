import { cn } from "@/lib/utils"

type Props = {
  color: string
  letter: string
  size?: "sm" | "md" | "lg"
  className?: string
}

const sizes = {
  sm: "size-7 text-[10px]",
  md: "size-9 text-[11px]",
  lg: "size-11 text-xs",
}

// A small colored letter tile that represents a source-system connector.
// Avoids needing brand SVGs while still being visually distinct.
export function ConnectorIcon({ color, letter, size = "md", className }: Props) {
  return (
    <span
      className={cn(
        "inline-flex items-center justify-center rounded-md font-semibold text-white tracking-tight shrink-0",
        sizes[size],
        className
      )}
      style={{ backgroundColor: color }}
      aria-hidden
    >
      {letter}
    </span>
  )
}
