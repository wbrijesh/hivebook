"use client"

import * as React from "react"
import Link from "next/link"
import { useRouter, useSearchParams } from "next/navigation"
import { RiArrowLeftLine, RiPhoneLine } from "@remixicon/react"
import { toast } from "sonner"

import {
  AuthCard,
  AuthCardHeader,
  AuthShell,
} from "@/components/auth/auth-shell"
import {
  InputOTP,
  InputOTPGroup,
  InputOTPSlot,
} from "@/components/ui/input-otp"
import { useAuthFlow } from "@/components/auth/auth-flow"
import { mockUser, MOCK_DELAY_MS } from "@/lib/mock-data"
import { cn } from "@/lib/utils"

const RESEND_SECONDS = 30

// Where a verified code leads, keyed by the flow that sent the user here:
//   signup → create a password   login → straight in   reset → set a new one
const NEXT_BY_FLOW: Record<string, string> = {
  signup: "/set-password",
  login: "/book",
  reset: "/set-password?mode=reset",
}

function VerifyEmailForm() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const flow = searchParams.get("flow") ?? "signup"
  const next = NEXT_BY_FLOW[flow] ?? NEXT_BY_FLOW.signup
  const { email } = useAuthFlow()
  const displayEmail = email || mockUser.email

  const [code, setCode] = React.useState("")
  const [resendIn, setResendIn] = React.useState(RESEND_SECONDS)
  const [verifying, setVerifying] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)

  React.useEffect(() => {
    if (resendIn <= 0) return
    const t = setInterval(() => setResendIn((s) => Math.max(0, s - 1)), 1000)
    return () => clearInterval(t)
  }, [resendIn])

  function handleComplete(value: string) {
    setVerifying(true)
    setError(null)
    setTimeout(() => {
      if (value === "000000") {
        setError("That code is incorrect. Try again or request a new one.")
        setVerifying(false)
        setCode("")
        // Put the cursor back on the first slot so the user can retype
        // immediately instead of having to click.
        setTimeout(() => {
          document.querySelector<HTMLInputElement>("[data-autofocus]")?.focus()
        }, 0)
      } else {
        router.push(next)
      }
    }, MOCK_DELAY_MS)
  }

  function handleResend() {
    setResendIn(RESEND_SECONDS)
    setError(null)
    toast.success("New code sent.", {
      description: `Sent to ${displayEmail}. Check your inbox.`,
    })
  }

  return (
    <AuthShell>
      <AuthCard>
        <AuthCardHeader
          title="Verify your email"
          subtitle={
            <>
              Enter the 6-digit code we sent to{" "}
              <span className="font-medium text-foreground">{displayEmail}</span>
              .
            </>
          }
        />

        <div className="flex flex-col items-center gap-4">
          <InputOTP
            maxLength={6}
            value={code}
            onChange={setCode}
            onComplete={handleComplete}
            disabled={verifying}
            data-autofocus
            aria-describedby="otp-status"
            aria-invalid={!!error}
          >
            <InputOTPGroup className={cn("gap-2.5", error && "[&_[data-slot=input-otp-slot]]:border-destructive")}>
              {[0, 1, 2, 3, 4, 5].map((i) => (
                <InputOTPSlot
                  key={i}
                  index={i}
                  className="size-12 rounded-lg border text-[16px] font-medium"
                />
              ))}
            </InputOTPGroup>
          </InputOTP>

          {/* Live region so screen readers hear the verifying/error status. */}
          <div id="otp-status" aria-live="polite" className="text-center">
            {verifying && (
              <p className="inline-flex items-center gap-2 text-[13px] text-muted-foreground">
                <span className="size-3.5 animate-spin rounded-full border-2 border-muted-foreground/30 border-t-foreground" />
                Verifying…
              </p>
            )}
            {error && !verifying && (
              <p role="alert" className="text-[13px] text-destructive">
                {error}
              </p>
            )}
          </div>

          <div className="text-center text-[13px] text-muted-foreground">
            {resendIn > 0 ? (
              <>
                Didn&apos;t receive a code? Resend in{" "}
                <span className="font-medium text-foreground">{resendIn}s</span>
              </>
            ) : (
              <>
                Didn&apos;t receive a code?{" "}
                <button
                  type="button"
                  onClick={handleResend}
                  className="font-medium text-foreground underline-offset-2 hover:underline"
                >
                  Resend now
                </button>
              </>
            )}
          </div>

          <button
            type="button"
            className="mt-1 inline-flex items-center gap-1.5 text-[12px] text-muted-foreground transition-colors hover:text-foreground"
            onClick={() => toast.info("We've called you with the code.")}
          >
            <RiPhoneLine className="size-3.5" />
            Send the code by phone instead
          </button>
        </div>
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

export default function VerifyEmailPage() {
  return (
    <React.Suspense>
      <VerifyEmailForm />
    </React.Suspense>
  )
}
