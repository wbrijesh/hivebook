"use client"

import * as React from "react"

import {
  DEFAULT_ACCENT_ID,
  DEFAULT_DARK_SHADE,
  DEFAULT_LIGHT_SHADE,
  getAccent,
  hoverShade,
  readableForeground,
} from "@/lib/accent-palettes"
import { DEFAULT_FONT_ID, getFont } from "@/lib/fonts"

type AccentConfig = {
  accentId: string
  lightShade: number
  darkShade: number
  fontId: string
}

type AccentContextValue = AccentConfig & {
  setAccentId: (id: string) => void
  setLightShade: (shade: number) => void
  setDarkShade: (shade: number) => void
  setFontId: (id: string) => void
  /** Whether the style picker control is shown — toggled with the "a" key. */
  pickerVisible: boolean
}

const STORAGE_KEY = "hivebook-accent"

const AccentContext = React.createContext<AccentContextValue | null>(null)

// Builds the CSS that overrides the accent tokens for both themes at once.
// Using a stylesheet (rather than inline styles on <html>) lets us target
// :root and .dark separately, so light/dark each get their own configured
// shade without us having to watch the theme toggle.
function buildCss(config: AccentConfig): string {
  const palette = getAccent(config.accentId)
  const l = palette.light[config.lightShade as keyof typeof palette.light]
  const d = palette.dark[config.darkShade as keyof typeof palette.dark]

  const light = {
    primary: l,
    primaryForeground: readableForeground(l),
    primaryHover: hoverShade(palette.light, config.lightShade),
    subtle: palette.light[100],
    subtleForeground: palette.light[1000],
  }
  const dark = {
    primary: d,
    primaryForeground: readableForeground(d),
    primaryHover: hoverShade(palette.dark, config.darkShade),
    subtle: palette.dark[100],
    subtleForeground: palette.dark[1100],
  }

  return `:root{--primary:${light.primary};--primary-foreground:${light.primaryForeground};--primary-hover:${light.primaryHover};--ring:${light.primary};--brand:${light.primary};--brand-foreground:${light.primaryForeground};--brand-subtle:${light.subtle};--brand-subtle-foreground:${light.subtleForeground};--sidebar-primary:${light.primary};--sidebar-primary-foreground:${light.primaryForeground};--sidebar-ring:${light.primary};}
.dark{--primary:${dark.primary};--primary-foreground:${dark.primaryForeground};--primary-hover:${dark.primaryHover};--ring:${dark.primary};--brand:${dark.primary};--brand-foreground:${dark.primaryForeground};--brand-subtle:${dark.subtle};--brand-subtle-foreground:${dark.subtleForeground};--sidebar-primary:${dark.primary};--sidebar-primary-foreground:${dark.primaryForeground};--sidebar-ring:${dark.primary};}`
}

export function AccentProvider({ children }: { children: React.ReactNode }) {
  const [config, setConfig] = React.useState<AccentConfig>({
    accentId: DEFAULT_ACCENT_ID,
    lightShade: DEFAULT_LIGHT_SHADE,
    darkShade: DEFAULT_DARK_SHADE,
    fontId: DEFAULT_FONT_ID,
  })

  // Hidden by default; press "a" to reveal (see the keydown handler below).
  const [pickerVisible, setPickerVisible] = React.useState(false)

  // Load any saved experiment on mount.
  React.useEffect(() => {
    try {
      const saved = localStorage.getItem(STORAGE_KEY)
      if (saved) setConfig((c) => ({ ...c, ...JSON.parse(saved) }))
    } catch {
      // ignore malformed storage
    }
  }, [])

  // Press "a" to show/hide the accent picker — but never while the user is
  // typing in a field or holding a modifier.
  React.useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key !== "a" && e.key !== "A") return
      if (e.metaKey || e.ctrlKey || e.altKey) return
      const t = e.target as HTMLElement | null
      if (
        t &&
        (t.isContentEditable ||
          ["INPUT", "TEXTAREA", "SELECT"].includes(t.tagName))
      ) {
        return
      }
      setPickerVisible((v) => !v)
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [])

  // Apply to the document + persist whenever the config changes.
  React.useEffect(() => {
    let el = document.getElementById("accent-theme") as HTMLStyleElement | null
    if (!el) {
      el = document.createElement("style")
      el.id = "accent-theme"
      document.head.appendChild(el)
    }
    el.textContent = buildCss(config)
    // Swap the active typeface by pointing --font-sans at the chosen font's var.
    document.documentElement.style.setProperty(
      "--font-sans",
      `var(${getFont(config.fontId).cssVar})`
    )
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(config))
    } catch {
      // ignore quota / privacy-mode errors
    }
  }, [config])

  const value = React.useMemo<AccentContextValue>(
    () => ({
      ...config,
      pickerVisible,
      setAccentId: (accentId) => setConfig((c) => ({ ...c, accentId })),
      setLightShade: (lightShade) => setConfig((c) => ({ ...c, lightShade })),
      setDarkShade: (darkShade) => setConfig((c) => ({ ...c, darkShade })),
      setFontId: (fontId) => setConfig((c) => ({ ...c, fontId })),
    }),
    [config, pickerVisible]
  )

  return (
    <AccentContext.Provider value={value}>{children}</AccentContext.Provider>
  )
}

export function useAccent() {
  const ctx = React.useContext(AccentContext)
  if (!ctx) throw new Error("useAccent must be used within AccentProvider")
  return ctx
}
