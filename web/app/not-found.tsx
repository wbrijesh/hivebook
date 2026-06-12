import Link from "next/link"

import { Button } from "@/components/ui/button"

export default function NotFound() {
  return (
    <main className="flex min-h-svh flex-col items-center justify-center gap-4 bg-background px-4 text-center">
      <h1 className="text-[18px] font-semibold tracking-tight text-foreground">
        Page not found
      </h1>
      <p className="text-[13px] text-muted-foreground">
        That page doesn&rsquo;t exist.
      </p>
      <Button asChild>
        <Link href="/">Go home</Link>
      </Button>
    </main>
  )
}
