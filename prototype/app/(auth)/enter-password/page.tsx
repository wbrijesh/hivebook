"use client"

import * as React from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import {
  RiArrowLeftLine,
  RiEyeLine,
  RiEyeOffLine,
  RiMailLine,
} from "@remixicon/react"

import {
  AuthCard,
  AuthCardHeader,
  AuthDivider,
  AuthField,
  AuthMethodButton,
  AuthShell,
  AuthSubmitButton,
  ButtonSpinner,
} from "@/components/auth/auth-shell"
import { useAuthFlow } from "@/components/auth/auth-flow"
import { Input } from "@/components/ui/input"
import { mockUser, MOCK_DELAY_MS } from "@/lib/mock-data"

// Returning-user step: the email is already known (we arrived here from the
// front door), so we only ask for the password — with a code-based escape
// hatch for anyone who'd rather not type it.
export default function EnterPasswordPage() {
  const router = useRouter()
  const { email } = useAuthFlow()
  const displayEmail = email || mockUser.email
  const [password, setPassword] = React.useState("")
  const [showPw, setShowPw] = React.useState(false)
  const [pending, setPending] = React.useState(false)

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!password) return
    setPending(true)
    setTimeout(() => router.push("/book"), MOCK_DELAY_MS)
  }

  return (
    <AuthShell>
      <AuthCard>
        <AuthCardHeader
          title="Enter your password"
          subtitle={
            <>
              Signing in as{" "}
              <span className="font-medium text-foreground">
                {displayEmail}
              </span>
              .
            </>
          }
        />

        <form onSubmit={handleSubmit} className="space-y-3">
          <AuthField
            label="Password"
            htmlFor="password"
            trailing={
              <Link
                href="/forgot-password"
                className="text-[12px] text-muted-foreground transition-colors hover:text-foreground"
              >
                Forgot password?
              </Link>
            }
          >
            <div className="relative">
              <Input
                id="password"
                type={showPw ? "text" : "password"}
                required
                data-autofocus
                placeholder="Enter your password"
                autoComplete="current-password"
                className="h-11 pr-10 text-[14px]"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
              <button
                type="button"
                onClick={() => setShowPw((v) => !v)}
                className="absolute right-2 top-1/2 inline-flex size-7 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                aria-label={showPw ? "Hide password" : "Show password"}
              >
                {showPw ? (
                  <RiEyeOffLine className="size-4" />
                ) : (
                  <RiEyeLine className="size-4" />
                )}
              </button>
            </div>
          </AuthField>

          <AuthSubmitButton disabled={pending} className="mt-1">
            {pending ? <ButtonSpinner label="Signing in…" /> : "Sign in"}
          </AuthSubmitButton>
        </form>

        <AuthDivider>or</AuthDivider>

        <AuthMethodButton
          icon={<RiMailLine className="size-4" />}
          label="Email me a code instead"
          onClick={() => router.push("/verify-email?flow=login")}
        />
      </AuthCard>

      <div className="mt-6 text-center">
        <Link
          href="/sign-in"
          className="inline-flex items-center gap-1 text-[13px] text-muted-foreground transition-colors hover:text-foreground"
        >
          <RiArrowLeftLine className="size-3.5" />
          Use a different email
        </Link>
      </div>
    </AuthShell>
  )
}
