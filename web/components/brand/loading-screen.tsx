import { Logo } from "@/components/brand/logo"

// Full-viewport placeholder shown while the session is resolving — a quiet mark,
// never a blank screen (design-language: no silent emptiness).
export function LoadingScreen() {
  return (
    <main className="flex min-h-svh items-center justify-center bg-background">
      <Logo
        variant="mark"
        size="lg"
        className="animate-pulse text-muted-foreground motion-reduce:animate-none"
      />
    </main>
  )
}
