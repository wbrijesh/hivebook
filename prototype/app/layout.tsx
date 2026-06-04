import {
  Geist,
  Geist_Mono,
  IBM_Plex_Sans,
  Inter,
  Public_Sans,
  Source_Sans_3,
} from "next/font/google"
import localFont from "next/font/local"

import "./globals.css"
import { ThemeProvider } from "@/components/theme-provider"
import { AccentProvider } from "@/components/brand/accent-provider"
import { Toaster } from "@/components/ui/sonner"
import { TooltipProvider } from "@/components/ui/tooltip"
import { cn } from "@/lib/utils";

// Candidate typefaces — each exposes its own CSS variable. The active one is
// chosen live by the style picker (sets --font-sans). Default is Source Sans 3
// (see --font-sans in globals.css). Keep in sync with lib/fonts.ts.
const fontSource = Source_Sans_3({ subsets: ["latin"], variable: "--font-source-sans", display: "swap" })
const fontInter = Inter({ subsets: ["latin"], variable: "--font-inter", display: "swap" })
const fontGeist = Geist({ subsets: ["latin"], variable: "--font-geist", display: "swap" })
const fontPublic = Public_Sans({ subsets: ["latin"], variable: "--font-public-sans", display: "swap" })
const fontPlex = IBM_Plex_Sans({
  subsets: ["latin"],
  weight: ["300", "400", "500", "600", "700"],
  variable: "--font-ibm-plex",
  display: "swap",
})
const fontDin = localFont({
  src: [
    { path: "./fonts/DIN2014-ExtraLight.ttf", weight: "200", style: "normal" },
    { path: "./fonts/DIN2014-Light.ttf", weight: "300", style: "normal" },
    { path: "./fonts/DIN2014-Regular.ttf", weight: "400", style: "normal" },
    { path: "./fonts/DIN2014-DemiBold.ttf", weight: "500", style: "normal" },
    { path: "./fonts/DIN2014-DemiBold.ttf", weight: "600", style: "normal" },
    { path: "./fonts/DIN2014-Bold.ttf", weight: "700", style: "normal" },
  ],
  variable: "--font-din",
  display: "swap",
})

const fontMono = Geist_Mono({
  subsets: ["latin"],
  variable: "--font-mono",
})

const fontVariables = [
  fontSource.variable,
  fontInter.variable,
  fontGeist.variable,
  fontPublic.variable,
  fontPlex.variable,
  fontDin.variable,
  fontMono.variable,
]

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
          <AccentProvider>
            <TooltipProvider delayDuration={150}>{children}</TooltipProvider>
            <Toaster position="bottom-center" />
          </AccentProvider>
        </ThemeProvider>
      </body>
    </html>
  )
}
