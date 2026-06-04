"use client"

import Link from "next/link"
import { RiArrowLeftLine } from "@remixicon/react"

import {
  AuthCard,
  AuthCardHeader,
  AuthMethodButton,
  AuthShell,
} from "@/components/auth/auth-shell"

export default function AuthErrorPage() {
  return (
    <AuthShell>
      <AuthCard>
        <AuthCardHeader
          title="We couldn't sign you in"
          subtitle="Something went wrong on our end. Try again, or use another method."
        />

        <div className="flex flex-col gap-2">
          <Link href="/sign-in" className="block">
            <AuthMethodButton
              icon={<RiArrowLeftLine className="size-4" />}
              label="Back to sign-in"
            />
          </Link>
          <Link href="/forgot-password" className="block">
            <AuthMethodButton
              icon={<RiArrowLeftLine className="size-4" />}
              label="Reset your password"
            />
          </Link>
        </div>

        <div className="mt-6 rounded-lg border border-border bg-muted/30 p-3">
          <p className="text-[12px] uppercase tracking-wider text-muted-foreground">
            Reference ID
          </p>
          <p className="mt-1 font-mono text-[12px] text-foreground">
            err_4f8a92e1c0d4b6e8
          </p>
          <p className="mt-2 text-[12px] text-muted-foreground">
            If you contact support, including this ID will help us find the
            problem quickly.
          </p>
        </div>
      </AuthCard>

      <div className="mt-6 text-center text-[13px] text-muted-foreground">
        Need help?{" "}
        <Link
          href="#"
          className="font-medium text-foreground underline-offset-2 hover:underline"
        >
          Contact support
        </Link>
      </div>
    </AuthShell>
  )
}
