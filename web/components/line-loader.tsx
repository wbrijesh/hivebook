import { cn } from "@/lib/utils"

// LineLoader is an indeterminate horizontal progress bar (Fluent/Material style): a
// single segment travels across the track while its width breathes and its opacity
// pulses (keyframes in globals.css). Use it where work is in flight with no known
// total — e.g. discovering a newly connected source's projects. Apex-tokened.
export function LineLoader({
  className,
  tone = "primary",
}: {
  className?: string
  // "primary" reads as active foreground work; "muted" is a quieter neutral.
  tone?: "primary" | "muted"
}) {
  return (
    <div
      role="progressbar"
      aria-label="Loading"
      className={cn(
        "relative h-[3px] w-full overflow-hidden rounded-full",
        tone === "primary" ? "bg-primary/15" : "bg-muted",
        className
      )}
    >
      <div
        className={cn(
          "hb-line-loader-bar absolute top-0 left-0 h-full w-[12%] rounded-full",
          tone === "primary" ? "bg-primary" : "bg-muted-foreground/70"
        )}
      />
    </div>
  )
}
