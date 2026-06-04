"use client"

import * as React from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { RiHotelLine } from "@remixicon/react"

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
import { GoogleLogo, MicrosoftLogo, PlayGlyph } from "@/components/icons"
import { Input } from "@/components/ui/input"
import { mockUser, MOCK_DELAY_MS } from "@/lib/mock-data"

// Google/Microsoft lead (the common cases); generic SSO follows.
const PROVIDERS = [
  { id: "microsoft", label: "Continue with Microsoft", icon: <MicrosoftLogo size={18} /> },
  { id: "google", label: "Continue with Google", icon: <GoogleLogo size={18} /> },
  {
    id: "sso",
    label: "Single sign-on (SSO)",
    icon: <RiHotelLine className="size-4" />,
    detail: "SAML · OIDC",
  },
]

export default function SignInPage() {
  const router = useRouter()
  const { email, setEmail, lastMethod, setLastMethod } = useAuthFlow()
  const [pending, setPending] = React.useState(false)

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!email) return
    setLastMethod("email")
    setPending(true)
    setTimeout(() => {
      setPending(false)
      // No backend yet, so we fake new-vs-returning recognition off the mock
      // identity: a known email goes to the password step; anything else is
      // treated as a new account and routed through email verification. The
      // typed email already lives in the flow store, so it follows along.
      if (email.trim().toLowerCase() === mockUser.email) {
        router.push("/enter-password")
      } else {
        router.push("/verify-email?flow=signup")
      }
    }, MOCK_DELAY_MS)
  }

  function useProvider(id: string, providerName: string) {
    setLastMethod(id)
    router.push(`/sso/callback?provider=${encodeURIComponent(providerName)}`)
  }

  // Float the last-used provider to the top so the returning user presses the
  // one that's already theirs instead of scanning four options.
  const ordered = [...PROVIDERS].sort(
    (a, b) => Number(b.id === lastMethod) - Number(a.id === lastMethod)
  )

  return (
    <AuthShell>
      <AuthCard>
        <AuthCardHeader
          title="Sign in or create your account"
          subtitle="Continue with your work email — we'll sign you in or get you set up."
        />

        {/* Primary path: email. The page is identity-first under the hood
            (we route on the address), so the UI leads with it — one obvious
            next action, with the only filled/accent control on the screen. */}
        <form onSubmit={handleSubmit} className="space-y-3">
          <AuthField label="Work email" htmlFor="email">
            <Input
              id="email"
              type="email"
              required
              placeholder="you@company.com"
              autoComplete="email"
              data-autofocus
              className="h-11 text-[14px]"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </AuthField>

          <AuthSubmitButton disabled={pending} className="mt-1">
            {pending ? (
              <ButtonSpinner label="Continuing…" />
            ) : (
              <>
                Continue
                <PlayGlyph className="size-4" />
              </>
            )}
          </AuthSubmitButton>
        </form>

        <AuthDivider>or continue with</AuthDivider>

        {/* Secondary path: identity providers — demoted below the divider, with
            the last-used one floated up and marked. */}
        <div className="space-y-2">
          {ordered.map((p) => {
            const last = p.id === lastMethod
            // The brand name (drop the "Continue with " prefix) for the loader.
            const providerName = p.label.replace("Continue with ", "")
            return (
              <AuthMethodButton
                key={p.id}
                icon={p.icon}
                label={p.label}
                detail={last ? "Last used" : p.detail}
                onClick={() => useProvider(p.id, providerName)}
              />
            )
          })}
        </div>
      </AuthCard>

      <p className="mt-6 text-center text-[13px] text-muted-foreground">
        By continuing you agree to the{" "}
        <Link href="#" className="underline-offset-2 hover:underline">
          Terms
        </Link>{" "}
        and{" "}
        <Link href="#" className="underline-offset-2 hover:underline">
          Privacy Policy
        </Link>
        .
      </p>
    </AuthShell>
  )
}
