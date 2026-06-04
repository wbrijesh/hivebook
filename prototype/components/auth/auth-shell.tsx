import * as React from "react"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

type AuthShellProps = {
  children: React.ReactNode
  /** Controls the max width of the centered content. */
  width?: "sm" | "md"
}

// The chrome (header, footer, background) lives in app/(auth)/layout.tsx so it
// stays fixed while pages transition. AuthShell only constrains the centered
// content width for a given page.
export function AuthShell({ children, width = "sm" }: AuthShellProps) {
  return (
    <div
      className={cn(
        "mx-auto w-full",
        width === "sm" ? "max-w-[420px]" : "max-w-[520px]"
      )}
    >
      {children}
    </div>
  )
}

// Surface card used to wrap auth forms. Subtle ring, soft white surface.
export function AuthCard({
  className,
  children,
}: {
  className?: string
  children: React.ReactNode
}) {
  return (
    <div
      className={cn(
        "rounded-2xl border border-border bg-card p-8 shadow-[0_1px_2px_0_rgb(0_0_0_/_0.02),0_8px_24px_-12px_rgb(0_0_0_/_0.05)]",
        className
      )}
    >
      {children}
    </div>
  )
}

export function AuthCardHeader({
  title,
  subtitle,
  align = "left",
}: {
  title: React.ReactNode
  subtitle?: React.ReactNode
  /** Form headers read better left-aligned; status/confirmation cards center. */
  align?: "left" | "center"
}) {
  return (
    <div className={cn("mb-7 space-y-1.5", align === "center" && "text-center")}>
      <h1 className="text-[18px] font-semibold tracking-tight text-foreground">
        {title}
      </h1>
      {subtitle && (
        <p className="text-[14px] leading-relaxed text-muted-foreground">
          {subtitle}
        </p>
      )}
    </div>
  )
}

// The pending-state spinner shown inside a primary button while a simulated
// action runs. Tinted to sit on the accent-colored primary button.
export function ButtonSpinner({ label }: { label: string }) {
  return (
    <span className="inline-flex items-center gap-2">
      <span className="size-3.5 animate-spin rounded-full border-2 border-primary-foreground/40 border-t-primary-foreground" />
      {label}
    </span>
  )
}

// Primary CTA — the customized shadcn Button at our tall, form-width size.
export function AuthSubmitButton({
  className,
  children,
  ...props
}: React.ComponentProps<typeof Button>) {
  return (
    <Button type="submit" size="xl" className={cn("w-full", className)} {...props}>
      {children}
    </Button>
  )
}

// Outline / secondary button at the same height.
export function AuthSecondaryButton({
  className,
  children,
  ...props
}: React.ComponentProps<typeof Button>) {
  return (
    <Button
      type="button"
      variant="outline"
      size="xl"
      className={cn("w-full", className)}
      {...props}
    >
      {children}
    </Button>
  )
}

// A row for an auth method (SSO provider, etc.): the outline Button, left
// aligned, with an icon, a label, and an optional trailing detail.
export function AuthMethodButton({
  icon,
  label,
  detail,
  onClick,
  className,
}: {
  icon: React.ReactNode
  label: string
  detail?: string
  onClick?: () => void
  className?: string
}) {
  return (
    <Button
      type="button"
      variant="secondary"
      size="xl"
      onClick={onClick}
      className={cn(
        "w-full justify-start gap-3 border border-border px-3.5 text-left",
        className
      )}
    >
      <span className="flex size-5 items-center justify-center text-foreground/80">
        {icon}
      </span>
      <span className="flex-1 text-foreground">{label}</span>
      {detail && (
        <span className="text-[12px] font-normal text-muted-foreground">
          {detail}
        </span>
      )}
    </Button>
  )
}

// A small divider with text in the middle. Used between auth method groups.
export function AuthDivider({ children }: { children: React.ReactNode }) {
  return (
    <div className="my-5 flex items-center gap-3 text-[12px] uppercase tracking-wider text-muted-foreground">
      <div className="h-px flex-1 bg-border" />
      <span>{children}</span>
      <div className="h-px flex-1 bg-border" />
    </div>
  )
}

// Field label + input wrapper for slightly taller inputs in auth flows.
export function AuthField({
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
