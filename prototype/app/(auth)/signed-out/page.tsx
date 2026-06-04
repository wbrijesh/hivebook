"use client"

import Link from "next/link"

import {
  AuthCard,
  AuthCardHeader,
  AuthShell,
  AuthSubmitButton,
} from "@/components/auth/auth-shell"

export default function SignedOutPage() {
  return (
    <AuthShell>
      <AuthCard className="text-center">
        <AuthCardHeader
          title="You're signed out"
          subtitle="We've ended your session on this device."
          align="center"
        />

        <Link href="/sign-in" className="block">
          <AuthSubmitButton>Sign back in</AuthSubmitButton>
        </Link>

        <p className="mt-5 text-center text-[12px] text-muted-foreground">
          On a shared computer? Close the browser too.
        </p>
      </AuthCard>
    </AuthShell>
  )
}
