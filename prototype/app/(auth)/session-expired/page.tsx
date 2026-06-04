"use client"

import Link from "next/link"

import {
  AuthCard,
  AuthCardHeader,
  AuthShell,
  AuthSubmitButton,
} from "@/components/auth/auth-shell"

export default function SessionExpiredPage() {
  return (
    <AuthShell>
      <AuthCard className="text-center">
        <AuthCardHeader
          title="Your session expired"
          subtitle="We sign you out after a period of inactivity. Sign in to continue where you left off."
          align="center"
        />

        <Link href="/sign-in" className="block">
          <AuthSubmitButton>Sign in again</AuthSubmitButton>
        </Link>

        <p className="mt-5 text-center text-[12px] text-muted-foreground">
          Your draft work has been saved.
        </p>
      </AuthCard>
    </AuthShell>
  )
}
