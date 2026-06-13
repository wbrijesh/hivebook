"use client"

import { useEffect } from "react"

import { Button } from "@/components/ui/button"
import { track } from "@/lib/telemetry"

// Root error boundary — a quiet, on-system fallback instead of Next's default
// (design-language: no silent emptiness).
export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string }
  reset: () => void
}) {
  // Don't let a render crash vanish: log it and beacon it to the telemetry sink.
  useEffect(() => {
    console.error(error)
    track("app_error", { message: error.message, digest: error.digest })
  }, [error])

  return (
    <main className="flex min-h-svh flex-col items-center justify-center gap-4 bg-background px-4 text-center">
      <h1 className="text-[18px] font-semibold tracking-tight text-foreground">
        Something went wrong
      </h1>
      <p className="max-w-sm text-[13px] text-muted-foreground">
        An unexpected error occurred. Try again, or reload the page.
      </p>
      <Button onClick={reset}>Try again</Button>
    </main>
  )
}
