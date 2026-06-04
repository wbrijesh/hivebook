// Accent palettes for the live theming control.
//
// Each family carries the full Adobe Spectrum scale (steps 100→1300) for both
// light and dark themes. The picker lets you choose a family and the exact
// shade used in each theme; the hover shade and the on-accent text color are
// derived from those choices (see accent-provider.tsx).

export const SHADES = [
  100, 200, 300, 400, 500, 600, 700, 800, 900, 1000, 1100, 1200, 1300,
] as const

export type Scale = Record<(typeof SHADES)[number], string>

export type AccentPalette = {
  id: string
  name: string
  light: Scale
  dark: Scale
}

function scale(values: string[]): Scale {
  return Object.fromEntries(SHADES.map((s, i) => [s, values[i]])) as Scale
}

export const ACCENTS: AccentPalette[] = [
  {
    id: "orange",
    name: "Orange",
    light: scale(["#ffeccc","#ffdfad","#fdd291","#ffbb63","#ffa037","#f68511","#e46f00","#cb5d00","#b14c00","#953d00","#7a2f00","#612300","#491901"]),
    dark: scale(["#662500","#752d00","#893700","#9e4200","#b44e00","#ca5d00","#e16d00","#f4810c","#fe9a2e","#ffb558","#fdce88","#ffe1b3","#fff2dd"]),
  },
  {
    id: "red",
    name: "Red",
    light: scale(["#ffebe7","#ffddd6","#ffcdc3","#ffb7a9","#ff9b88","#ff7c65","#f75c46","#ea3829","#d31510","#b40000","#930000","#740000","#590000"]),
    dark: scale(["#7b0000","#8d0000","#a50000","#be0403","#d71913","#ea3829","#f65843","#ff755e","#ff9581","#ffb0a1","#ffc9bd","#ffded8","#fff1ee"]),
  },
  {
    id: "orange-yellow",
    name: "Yellow",
    light: scale(["#fbf198","#f8e750","#f8d904","#e8c600","#d7b300","#c49f00","#b08c00","#9b7800","#856600","#705300","#5b4300","#483300","#362500"]),
    dark: scale(["#4c3600","#584000","#674c00","#775900","#886800","#9b7800","#ae8900","#c09c00","#d3ae00","#e4c200","#f4d500","#f9e85c","#fcf6bb"]),
  },
  {
    id: "green",
    name: "Green",
    light: scale(["#cef8e0","#adf4ce","#89ecbc","#67dea8","#49cc93","#2fb880","#15a46e","#008f5d","#007a4d","#00653e","#005132","#053f27","#0a2e1d"]),
    dark: scale(["#044329","#004e2f","#005c38","#006c43","#007d4e","#008f5d","#12a26c","#2bb47d","#43c78f","#5ed9a2","#81e9b8","#b1f4d1","#dffaea"]),
  },
  {
    id: "seafoam",
    name: "Seafoam",
    light: scale(["#cef7f3","#aaf1ea","#8ce9e2","#65dad2","#3fc9c1","#0fb5ae","#00a19a","#008c87","#007772","#00635f","#0c4f4c","#123c3a","#122c2b"]),
    dark: scale(["#12413f","#0e4c49","#045a57","#006965","#007a75","#008c87","#009e98","#03b2ab","#36c5bd","#5dd6cf","#84e6df","#b0f2ec","#dff9f6"]),
  },
  {
    id: "cyan",
    name: "Cyan",
    light: scale(["#c5f8ff","#a4f0ff","#88e7fa","#60d8f3","#33c5e8","#12b0da","#019cc8","#0086b4","#00719f","#005d89","#004a73","#00395d","#002a46"]),
    dark: scale(["#003d62","#00476f","#00557f","#006491","#0074a2","#0086b4","#0099c6","#0eadd7","#2cc1e6","#54d3f1","#7fe4f9","#a7f1ff","#d7faff"]),
  },
  {
    id: "blue",
    name: "Blue",
    light: scale(["#e0f2ff","#cae8ff","#b5deff","#96cefd","#78bbfa","#59a7f6","#3892f3","#147af3","#0265dc","#0054b6","#004491","#003571","#002754"]),
    dark: scale(["#003877","#00418a","#004da3","#0059c2","#0367e0","#1379f3","#348ff4","#54a3f6","#72b7f9","#8fcafc","#aedbfe","#cce9ff","#e8f6ff"]),
  },
  {
    id: "indigo",
    name: "Indigo",
    light: scale(["#edeeff","#e0e2ff","#d3d5ff","#c1c4ff","#acafff","#9599ff","#7e84fc","#686df4","#5258e4","#4046ca","#3236a8","#262986","#1b1e64"]),
    dark: scale(["#282c8c","#2f34a3","#393fbb","#464bd3","#555be7","#686df4","#7c81fb","#9195ff","#a7aaff","#bcbeff","#d0d2ff","#e2e4ff","#f3f3fe"]),
  },
  {
    id: "purple",
    name: "Purple",
    light: scale(["#f6ebff","#eeddff","#e6d0ff","#dbbbfe","#cca4fd","#bd8bfc","#ae72f9","#9d57f4","#893de7","#7326d3","#5d13b7","#470c94","#33106a"]),
    dark: scale(["#4c0d9d","#5911b1","#691cc8","#7a2dda","#8c41e9","#9d57f3","#ac6ff9","#bb87fb","#ca9ffc","#d7b6fe","#e4ccfe","#efdfff","#f9f0ff"]),
  },
  {
    id: "fuchsia",
    name: "Fuchsia",
    light: scale(["#ffe9fc","#ffdafa","#fec7f8","#fbaef6","#f592f3","#ed74ed","#e055e2","#cd3ace","#b622b7","#9d039e","#800081","#640664","#470e46"]),
    dark: scale(["#6b036a","#7b007b","#900091","#a50da6","#b925b9","#cd39ce","#df51e0","#eb6eec","#f48cf2","#faa8f5","#fec2f8","#ffdbfa","#ffeffc"]),
  },
  {
    id: "magenta",
    name: "Magenta",
    light: scale(["#ffeaf1","#ffdce8","#ffcadd","#ffb2ce","#ff95bd","#fa77aa","#ef5a98","#de3d82","#c82269","#ad0955","#8e0045","#700037","#54032a"]),
    dark: scale(["#76003a","#890042","#a0004d","#b6125a","#cb266d","#de3d82","#ed5795","#f972a7","#ff8fb9","#ffacca","#ffc6da","#ffdde9","#fff0f5"]),
  },
]

export const DEFAULT_ACCENT_ID = "orange"
export const DEFAULT_LIGHT_SHADE = 900
export const DEFAULT_DARK_SHADE = 900

export function getAccent(id: string): AccentPalette {
  return ACCENTS.find((a) => a.id === id) ?? ACCENTS[0]
}

// Relative luminance (WCAG) → choose near-black or white text on the accent.
export function readableForeground(hex: string): string {
  const c = hex.replace("#", "")
  const rgb = [0, 2, 4].map((i) => parseInt(c.slice(i, i + 2), 16) / 255)
  const lin = rgb.map((v) =>
    v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4)
  )
  const L = 0.2126 * lin[0] + 0.7152 * lin[1] + 0.0722 * lin[2]
  return L > 0.45 ? "#1d1d1d" : "#ffffff"
}

// The hover shade is one step along the scale. In light mode that means a
// darker step; in dark mode the accent is already bright, so one step the
// other way keeps a visible-but-subtle shift. We just move one index toward
// the middle so there's always a real neighbor to land on.
export function hoverShade(scaleObj: Scale, shade: number): string {
  const idx = SHADES.indexOf(shade as (typeof SHADES)[number])
  const nextIdx = idx >= SHADES.length - 1 ? idx - 1 : idx + 1
  return scaleObj[SHADES[nextIdx]]
}
