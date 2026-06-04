"use client"

import * as React from "react"
import { RiArrowDownSLine, RiCheckLine } from "@remixicon/react"

import { useAccent } from "@/components/brand/accent-provider"
import { ACCENTS, getAccent, hoverShade, SHADES } from "@/lib/accent-palettes"
import { FONTS } from "@/lib/fonts"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import { cn } from "@/lib/utils"

function Swatch({ color, className }: { color: string; className?: string }) {
  return (
    <span
      className={cn(
        "inline-block size-4 shrink-0 rounded-[5px] ring-1 ring-inset ring-black/15",
        className
      )}
      style={{ backgroundColor: color }}
    />
  )
}

// DEV ONLY. A combined style picker (typeface + accent) for dialing in the
// look. Hidden until the "a" key is pressed (see AccentProvider).
export function StylePicker() {
  const {
    accentId,
    lightShade,
    darkShade,
    fontId,
    setAccentId,
    setLightShade,
    setDarkShade,
    setFontId,
    pickerVisible,
  } = useAccent()
  const active = getAccent(accentId)

  if (!pickerVisible) return null

  return (
    <Popover>
      <PopoverTrigger className="inline-flex items-center gap-1.5 rounded-md border border-border bg-background px-2 py-1 text-[13px] text-foreground transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
        <Swatch color={active.light[lightShade as keyof typeof active.light]} />
        <span className="font-medium">Style</span>
        <RiArrowDownSLine className="size-3.5 text-muted-foreground" />
      </PopoverTrigger>

      <PopoverContent align="end" className="w-72 p-3">
        <SectionLabel>Typeface</SectionLabel>
        <div className="space-y-0.5">
          {FONTS.map((f) => {
            const selected = f.id === fontId
            return (
              <button
                key={f.id}
                type="button"
                onClick={() => setFontId(f.id)}
                className={cn(
                  "flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left transition-colors hover:bg-muted",
                  selected && "bg-muted"
                )}
              >
                <span
                  className="min-w-0 flex-1 truncate text-[14px] text-foreground"
                  style={{ fontFamily: `var(${f.cssVar})` }}
                >
                  {f.label}
                </span>
                <span className="shrink-0 text-[11px] text-muted-foreground">{f.note}</span>
                {selected ? (
                  <RiCheckLine className="size-3.5 shrink-0 text-foreground" />
                ) : (
                  <span className="size-3.5 shrink-0" />
                )}
              </button>
            )
          })}
        </div>

        <div className="my-3 border-t border-border" />

        <SectionLabel>Accent color</SectionLabel>
        <div className="grid grid-cols-2 gap-1">
          {ACCENTS.map((a) => {
            const selected = a.id === accentId
            return (
              <button
                key={a.id}
                type="button"
                onClick={() => setAccentId(a.id)}
                className={cn(
                  "flex items-center gap-2 rounded-md px-2 py-1.5 text-left text-[13px] transition-colors hover:bg-muted",
                  selected && "bg-muted"
                )}
              >
                <Swatch color={a.light[900]} />
                <span className="flex-1 truncate text-foreground">{a.name}</span>
                {selected && <RiCheckLine className="size-3.5 text-foreground" />}
              </button>
            )
          })}
        </div>

        <ShadeRow label="Light shade" scaleObj={active.light} value={lightShade} onChange={setLightShade} />
        <ShadeRow label="Dark shade" scaleObj={active.dark} value={darkShade} onChange={setDarkShade} />

        <p className="mt-3 text-[12px] leading-relaxed text-muted-foreground">
          Hover ({hoverLabel(active, lightShade, darkShade)}) and the text color on
          the accent are derived automatically.
        </p>
      </PopoverContent>
    </Popover>
  )
}

function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <p className="mb-1.5 text-[12px] font-medium uppercase tracking-wide text-muted-foreground">
      {children}
    </p>
  )
}

function ShadeRow({
  label,
  scaleObj,
  value,
  onChange,
}: {
  label: string
  scaleObj: Record<number, string>
  value: number
  onChange: (shade: number) => void
}) {
  return (
    <div className="mt-3 border-t border-border pt-3">
      <div className="mb-1.5 flex items-center justify-between">
        <span className="text-[12px] font-medium uppercase tracking-wide text-muted-foreground">
          {label}
        </span>
        <span className="font-mono text-[12px] text-muted-foreground">{value}</span>
      </div>
      <div className="flex items-center gap-1">
        {SHADES.map((s) => (
          <button
            key={s}
            type="button"
            aria-label={`${label} ${s}`}
            onClick={() => onChange(s)}
            className={cn(
              "size-5 flex-1 rounded-[5px] ring-1 ring-inset ring-black/15 transition-transform",
              value === s && "ring-2 ring-foreground ring-offset-1 ring-offset-background"
            )}
            style={{ backgroundColor: scaleObj[s] }}
          />
        ))}
      </div>
    </div>
  )
}

function hoverLabel(
  active: ReturnType<typeof getAccent>,
  lightShade: number,
  darkShade: number
) {
  const l = hoverShade(active.light, lightShade)
  const d = hoverShade(active.dark, darkShade)
  return l === d ? l : `${l} / ${d}`
}
