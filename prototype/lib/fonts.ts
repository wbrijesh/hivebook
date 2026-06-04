// Candidate UI typefaces offered by the dev style picker. Each maps to a CSS
// variable defined by next/font in app/layout.tsx; the picker sets --font-sans
// to the chosen one. All are OFL/Google except DIN 2014 (separately licensed).

export type FontOption = {
  id: string
  label: string
  cssVar: string
  note: string
}

export const FONTS: FontOption[] = [
  { id: "source-sans", label: "Source Sans 3", cssVar: "--font-source-sans", note: "Adobe · humanist" },
  { id: "inter", label: "Inter", cssVar: "--font-inter", note: "neutral workhorse" },
  { id: "geist", label: "Geist", cssVar: "--font-geist", note: "Vercel · geometric" },
  { id: "din", label: "DIN 2014", cssVar: "--font-din", note: "licensed · technical" },
  { id: "ibm-plex", label: "IBM Plex Sans", cssVar: "--font-ibm-plex", note: "IBM · engineered" },
  { id: "public-sans", label: "Public Sans", cssVar: "--font-public-sans", note: "USWDS · plain" },
]

export const DEFAULT_FONT_ID = "source-sans"

export function getFont(id: string): FontOption {
  return FONTS.find((f) => f.id === id) ?? FONTS[0]
}
