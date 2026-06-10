"use client"

import * as React from "react"
import { usePathname } from "next/navigation"

import { Button } from "@/components/ui/button"
import { PlayGlyph } from "@/components/icons"
import { currentStep } from "@/lib/onboarding-steps"
import { cn } from "@/lib/utils"

const widths = {
  md: "max-w-[560px]",
  lg: "max-w-[860px]",
}

// Per-step frame: a left-aligned header (title + one-line why, pulled from the
// step registry), a scrollable content column, and a footer with a quiet Back
// and an accent Continue. The top bar and progress live in the layout.
export function OnboardingShell({
  width = "md",
  secondaryAction,
  primaryAction,
  children,
}: {
  width?: "md" | "lg"
  secondaryAction?: React.ReactNode
  primaryAction?: React.ReactNode
  children: React.ReactNode
}) {
  const pathname = usePathname()
  const step = currentStep(pathname)
  const w = widths[width]

  return (
    <div className="flex h-full flex-col">
      <div className="shrink-0 pt-9 pb-5">
        <div className={cn("mx-auto w-full px-6", w)}>
          <h1 className="text-[18px] font-semibold tracking-tight text-foreground">
            {step?.title}
          </h1>
          {step?.why && (
            <p className="mt-1.5 text-[13px] leading-relaxed text-muted-foreground">
              {step.why}
            </p>
          )}
        </div>
      </div>

      <div className="flex-1 overflow-y-auto">
        <div className={cn("mx-auto w-full px-6 pb-8", w)}>{children}</div>
      </div>

      {(secondaryAction || primaryAction) && (
        <div className="shrink-0 border-t border-border bg-background">
          <div
            className={cn(
              "mx-auto flex w-full items-center justify-between gap-3 px-6 py-4",
              w
            )}
          >
            <div className="flex items-center">{secondaryAction}</div>
            <div className="flex items-center">{primaryAction}</div>
          </div>
        </div>
      )}
    </div>
  )
}

// Quiet, secondary. No skip — Back is the only way out, and it's understated.
export function OnboardingBack({ onClick }: { onClick: () => void }) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="xl"
      onClick={onClick}
      className="px-3 text-muted-foreground"
    >
      Back
    </Button>
  )
}

// The single forward action. Carries the play glyph (forward-progress motif).
export function OnboardingContinue({
  onClick,
  disabled,
  children = "Continue",
}: {
  onClick: () => void
  disabled?: boolean
  children?: React.ReactNode
}) {
  return (
    <Button
      type="button"
      size="xl"
      onClick={onClick}
      disabled={disabled}
      className="px-6"
    >
      {children}
      <PlayGlyph className="size-4" />
    </Button>
  )
}
