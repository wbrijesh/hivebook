"use client"

import * as React from "react"
import { cn } from "@/lib/utils"

// Auth-only background: a dense field of small dots, revealed as a soft
// circular gradient rising from the bottom-center. Quiet and anchored low so
// it never competes with the card; a faint brand-tinted spotlight follows the
// cursor. Sits behind the opaque card/header/footer, so it only shows in the
// gaps. Used on the auth screens only — not the index or onboarding.
const DOT_SIZE = "18px 18px"

export function AuthBgPattern({ className }: { className?: string }) {
  const ref = React.useRef<HTMLDivElement>(null)

  React.useEffect(() => {
    const el = ref.current
    if (!el) return
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return

    let frame = 0
    function onMove(e: PointerEvent) {
      if (!el) return
      cancelAnimationFrame(frame)
      frame = requestAnimationFrame(() => {
        const rect = el.getBoundingClientRect()
        el.style.setProperty("--x", `${e.clientX - rect.left}px`)
        el.style.setProperty("--y", `${e.clientY - rect.top}px`)
        el.style.setProperty("--spot", "1")
      })
    }
    window.addEventListener("pointermove", onMove)
    return () => {
      window.removeEventListener("pointermove", onMove)
      cancelAnimationFrame(frame)
    }
  }, [])

  return (
    <div
      ref={ref}
      aria-hidden
      className={cn(
        "pointer-events-none absolute inset-0 -z-10 overflow-hidden",
        className
      )}
      style={
        { "--x": "50%", "--y": "50%", "--spot": "0" } as React.CSSProperties
      }
    >
      {/* Dense neutral dot field, masked into a circular glow at the bottom.
          Light dots on a dark background read stronger than dark-on-light, so
          we dim the field further in dark mode to keep it equally quiet. */}
      <div
        className="absolute inset-0 dark:opacity-45"
        style={{
          backgroundImage:
            "radial-gradient(circle at center, color-mix(in srgb, var(--foreground) 8%, transparent) 1.4px, transparent 2px)",
          backgroundSize: DOT_SIZE,
          maskImage:
            "radial-gradient(circle at 50% 100%, black 0%, transparent 82%)",
          WebkitMaskImage:
            "radial-gradient(circle at 50% 100%, black 0%, transparent 82%)",
        }}
      />
      {/* Brand-tinted dots revealed in a soft radius around the cursor. */}
      <div
        className="absolute inset-0 transition-opacity duration-500"
        style={{
          opacity: "calc(var(--spot) * 0.1)",
          backgroundImage:
            "radial-gradient(circle at center, var(--brand) 1.4px, transparent 2px)",
          backgroundSize: DOT_SIZE,
          maskImage:
            "radial-gradient(220px 220px at var(--x) var(--y), black 0%, transparent 70%)",
          WebkitMaskImage:
            "radial-gradient(220px 220px at var(--x) var(--y), black 0%, transparent 70%)",
        }}
      />
    </div>
  )
}
