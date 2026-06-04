"use client"

import * as React from "react"
import Link from "next/link"
import { useRouter, useSearchParams } from "next/navigation"
import {
  RiArrowLeftLine,
  RiCheckLine,
  RiCloseLine,
  RiEyeLine,
  RiEyeOffLine,
} from "@remixicon/react"

import {
  AuthCard,
  AuthCardHeader,
  AuthField,
  AuthShell,
  AuthSubmitButton,
  ButtonSpinner,
} from "@/components/auth/auth-shell"
import { Input } from "@/components/ui/input"
import { Checkbox } from "@/components/ui/checkbox"
import { MOCK_DELAY_MS } from "@/lib/mock-data"
import { cn } from "@/lib/utils"

// This page serves two flows that share identical UI:
//   - create (default): first-time password creation, reached after the OTP
//     step in the email-first sign-up. Continues into onboarding.
//   - reset (?mode=reset): post-forgot-password reset. Returns to sign-in.
function SetPasswordForm() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const isReset = searchParams.get("mode") === "reset"

  const [pw, setPw] = React.useState("")
  const [pw2, setPw2] = React.useState("")
  const [showPw, setShowPw] = React.useState(false)
  const [pending, setPending] = React.useState(false)
  const [tried, setTried] = React.useState(false)

  // Length-led (per current NIST guidance): a 12+ character password matching
  // its confirmation is all we require. Character variety is encouraged via the
  // strength meter, not mandated — fewer arbitrary rules, less friction.
  const MIN_LENGTH = 12
  const lengthOk = pw.length >= MIN_LENGTH
  const match = pw2.length > 0 && pw === pw2
  const ok = lengthOk && match

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    // Button stays enabled; on an invalid submit we surface the unmet
    // requirements rather than silently greying the button out.
    if (!ok) {
      setTried(true)
      return
    }
    setPending(true)
    setTimeout(
      () => router.push(isReset ? "/sign-in" : "/onboarding/organization"),
      MOCK_DELAY_MS
    )
  }

  return (
    <AuthShell>
      <AuthCard>
        <AuthCardHeader
          title={isReset ? "Set a new password" : "Create your password"}
          subtitle={
            isReset
              ? "Choose a strong password. You'll be signed out from other devices."
              : "This secures your account. You'll use it next time you sign in."
          }
        />

        <form onSubmit={handleSubmit} className="space-y-4">
          <AuthField
            label={isReset ? "New password" : "Password"}
            htmlFor="pw"
          >
            <div className="relative">
              <Input
                id="pw"
                type={showPw ? "text" : "password"}
                required
                autoComplete="new-password"
                placeholder="At least 12 characters"
                className="h-11 pr-10 text-[14px]"
                value={pw}
                onChange={(e) => setPw(e.target.value)}
              />
              <button
                type="button"
                onClick={() => setShowPw((v) => !v)}
                className="absolute right-2 top-1/2 inline-flex size-7 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                aria-label={showPw ? "Hide password" : "Show password"}
              >
                {showPw ? <RiEyeOffLine className="size-4" /> : <RiEyeLine className="size-4" />}
              </button>
            </div>
          </AuthField>

          <AuthField
            label={isReset ? "Confirm new password" : "Confirm password"}
            htmlFor="pw2"
          >
            <div className="relative">
              <Input
                id="pw2"
                type={showPw ? "text" : "password"}
                required
                autoComplete="new-password"
                placeholder="Type it again"
                className="h-11 pr-10 text-[14px]"
                value={pw2}
                onChange={(e) => setPw2(e.target.value)}
              />
              <button
                type="button"
                onClick={() => setShowPw((v) => !v)}
                className="absolute right-2 top-1/2 inline-flex size-7 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                aria-label={showPw ? "Hide password" : "Show password"}
              >
                {showPw ? <RiEyeOffLine className="size-4" /> : <RiEyeLine className="size-4" />}
              </button>
            </div>
          </AuthField>

          {pw.length > 0 && <StrengthMeter pw={pw} />}

          <ul className="space-y-1 text-[12px] text-muted-foreground">
            <Rule ok={lengthOk} failed={tried && !lengthOk}>
              At least 12 characters
            </Rule>
            <Rule ok={match} failed={tried && !match}>
              Passwords match
            </Rule>
          </ul>

          {isReset && (
            <label className="flex items-start gap-2.5 pt-1">
              <Checkbox id="logout" defaultChecked className="mt-0.5" />
              <span className="text-[13px] leading-relaxed text-muted-foreground">
                Sign me out of all other devices and active sessions.
              </span>
            </label>
          )}

          <AuthSubmitButton disabled={pending} className="mt-1">
            {pending ? (
              <ButtonSpinner label={isReset ? "Updating…" : "Creating account…"} />
            ) : isReset ? (
              "Update password"
            ) : (
              "Create account"
            )}
          </AuthSubmitButton>
        </form>
      </AuthCard>

      <div className="mt-6 text-center">
        <Link
          href="/sign-in"
          className="inline-flex items-center gap-1 text-[13px] text-muted-foreground transition-colors hover:text-foreground"
        >
          <RiArrowLeftLine className="size-3.5" />
          Back to sign-in
        </Link>
      </div>
    </AuthShell>
  )
}

function Rule({
  ok,
  failed,
  children,
}: {
  ok: boolean
  failed?: boolean
  children: React.ReactNode
}) {
  return (
    <li className="flex items-center gap-1.5">
      {ok ? (
        <RiCheckLine className="size-3 text-success" />
      ) : (
        <RiCloseLine
          className={cn(
            "size-3",
            failed ? "text-destructive" : "text-muted-foreground/50"
          )}
        />
      )}
      <span
        className={cn(ok && "text-foreground", failed && !ok && "text-destructive")}
      >
        {children}
      </span>
    </li>
  )
}

// Length-led strength feedback (encouragement, not a gate).
function StrengthMeter({ pw }: { pw: string }) {
  const variety = [/[a-z]/, /[A-Z]/, /\d/, /[^a-zA-Z0-9]/].filter((r) =>
    r.test(pw)
  ).length
  let score = 0
  if (pw.length >= 8) score++
  if (pw.length >= 12) score++
  if (pw.length >= 16) score++
  if (variety >= 3) score++
  score = Math.min(4, score)

  const label = ["", "Weak", "Fair", "Good", "Strong"][score]
  const color =
    score <= 1 ? "bg-destructive" : score === 2 ? "bg-warning" : "bg-success"

  return (
    <div className="space-y-1.5">
      <div className="flex gap-1">
        {[0, 1, 2, 3].map((i) => (
          <span
            key={i}
            className={cn(
              "h-1 flex-1 rounded-full",
              i < score ? color : "bg-muted"
            )}
          />
        ))}
      </div>
      {label && (
        <p className="text-[12px] text-muted-foreground">
          Strength:{" "}
          <span className="font-medium text-foreground">{label}</span>
        </p>
      )}
    </div>
  )
}

export default function SetPasswordPage() {
  return (
    <React.Suspense>
      <SetPasswordForm />
    </React.Suspense>
  )
}
