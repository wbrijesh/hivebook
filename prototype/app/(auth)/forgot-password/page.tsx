"use client"

import * as React from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { RiArrowLeftLine } from "@remixicon/react"

import {
  AuthCard,
  AuthCardHeader,
  AuthField,
  AuthShell,
  AuthSubmitButton,
  ButtonSpinner,
} from "@/components/auth/auth-shell"
import { useAuthFlow } from "@/components/auth/auth-flow"
import { Input } from "@/components/ui/input"
import { MOCK_DELAY_MS } from "@/lib/mock-data"

export default function ForgotPasswordPage() {
  const router = useRouter()
  // Pre-filled with the email already in the flow (e.g. arriving from the
  // password step) so the user isn't asked to re-type what we just showed them.
  const { email, setEmail } = useAuthFlow()
  const [pending, setPending] = React.useState(false)

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!email) return
    setPending(true)
    setTimeout(() => router.push("/verify-email?flow=reset"), MOCK_DELAY_MS)
  }

  return (
    <AuthShell>
      <AuthCard>
        <AuthCardHeader
          title="Reset your password"
          subtitle="Enter the email associated with your account and we'll send you a 6-digit reset code."
        />

        <form onSubmit={handleSubmit} className="space-y-3">
          <AuthField label="Work email" htmlFor="email">
            <Input
              id="email"
              type="email"
              required
              autoComplete="email"
              placeholder="you@company.com"
              className="h-11 text-[14px]"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </AuthField>

          <AuthSubmitButton disabled={pending}>
            {pending ? (
              <ButtonSpinner label="Sending code…" />
            ) : (
              "Send reset code"
            )}
          </AuthSubmitButton>
        </form>

        <p className="mt-5 text-[12px] text-muted-foreground">
          If your organization uses Workspace SSO, password reset is managed by your
          identity provider. You may not receive a reset code from us.
        </p>
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
