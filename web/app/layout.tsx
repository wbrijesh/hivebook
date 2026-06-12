import type { Metadata } from "next"
import { Geist_Mono, Inter } from "next/font/google"

import "./globals.css"
import { ThemeProvider } from "@/components/theme-provider"
import { Toaster } from "@/components/ui/sonner"
import { TooltipProvider } from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"

// The UI typeface (Inter) and the mono face. Each exposes a CSS variable that
// the design tokens read (--font-sans → --font-inter; see app/globals.css).
const fontInter = Inter({
  subsets: ["latin"],
  variable: "--font-inter",
  display: "swap",
})
const fontMono = Geist_Mono({ subsets: ["latin"], variable: "--font-mono" })

const fontVariables = [fontInter.variable, fontMono.variable]

export const metadata: Metadata = {
  title: { default: "Hivebook", template: "%s · Hivebook" },
  description: "Your company's brain.",
}

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <html
      lang="en"
      suppressHydrationWarning
      className={cn("antialiased", "font-sans", ...fontVariables)}
    >
      <body>
        <ThemeProvider>
          <TooltipProvider delayDuration={150}>{children}</TooltipProvider>
          <Toaster position="bottom-center" />
        </ThemeProvider>
      </body>
    </html>
  )
}
