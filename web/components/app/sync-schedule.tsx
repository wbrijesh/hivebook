"use client"

import { useState } from "react"

import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"
import { cn } from "@/lib/utils"

// TEMP dev floor: 1 minute. Mirrors the server's minSyncIntervalSeconds — raise both
// to 900 (15 min) before GA.
const MIN_SECONDS = 60

const PRESETS = [
  { label: "15 min", seconds: 900 },
  { label: "1 hour", seconds: 3600 },
  { label: "6 hours", seconds: 21600 },
  { label: "24 hours", seconds: 86400 },
]

const UNITS = [
  { label: "min", seconds: 60 },
  { label: "hr", seconds: 3600 },
  { label: "day", seconds: 86400 },
]

// decompose seconds into the cleanest {value, unit} for the custom editor: the
// largest unit that divides evenly (86400 → 1 day, 5400 → 90 min, 7200 → 2 hr).
function decompose(seconds: number): { value: number; unit: number } {
  for (const u of [...UNITS].reverse()) {
    if (seconds % u.seconds === 0)
      return { value: seconds / u.seconds, unit: u.seconds }
  }
  return { value: Math.max(1, Math.round(seconds / 60)), unit: 60 }
}

// Segmented is the iOS/Linear pill group: exactly one raised, selected segment; the
// rest are quiet until hovered. A real radiogroup, not a styled <select>.
function Segmented<T extends string | number>({
  options,
  value,
  onChange,
  ariaLabel,
}: {
  options: { label: string; value: T }[]
  value: T
  onChange: (v: T) => void
  ariaLabel: string
}) {
  return (
    <div
      role="radiogroup"
      aria-label={ariaLabel}
      className="inline-flex items-center gap-0.5 rounded-lg border border-border bg-muted/40 p-0.5"
    >
      {options.map((o) => {
        const active = o.value === value
        return (
          <button
            key={String(o.value)}
            type="button"
            role="radio"
            aria-checked={active}
            onClick={() => onChange(o.value)}
            className={cn(
              "rounded-md px-2.5 py-1 text-[13px] font-medium transition-colors",
              active
                ? "bg-background text-foreground shadow-sm"
                : "text-muted-foreground hover:text-foreground"
            )}
          >
            {o.label}
          </button>
        )
      })}
    </div>
  )
}

// SyncSchedule is the per-source auto-sync control: a switch, preset frequencies, and
// an advanced custom value + unit. Controlled — it reports the chosen {enabled,
// seconds} up; the parent owns the saved/pending values and the Save action.
export function SyncSchedule({
  enabled,
  intervalSeconds,
  onEnabledChange,
  onIntervalChange,
}: {
  enabled: boolean
  intervalSeconds: number
  onEnabledChange: (enabled: boolean) => void
  onIntervalChange: (seconds: number) => void
}) {
  const matched = PRESETS.find((p) => p.seconds === intervalSeconds)
  // Custom mode is sticky once entered (and starts on when the saved value isn't a
  // preset), so an interval that happens to equal a preset doesn't yank the user out
  // of the editor mid-edit.
  const [custom, setCustom] = useState(!matched)
  const initial = decompose(intervalSeconds)
  const [value, setValue] = useState(String(initial.value))
  const [unit, setUnit] = useState(initial.unit)

  // Report up only when the custom value is valid (≥ the floor); otherwise the parent
  // keeps the last valid interval and the red hint flags it.
  function applyCustom(rawValue: string, unitSeconds: number) {
    setValue(rawValue)
    setUnit(unitSeconds)
    const n = Number(rawValue)
    if (!Number.isFinite(n) || n <= 0) return
    const seconds = Math.round(n * unitSeconds)
    if (seconds < MIN_SECONDS) return
    onIntervalChange(seconds)
  }

  const customSeconds = Math.round(Number(value) * unit)
  const belowMin =
    custom && (!Number.isFinite(customSeconds) || customSeconds < MIN_SECONDS)

  const freqOptions: { label: string; value: number | "custom" }[] = [
    ...PRESETS.map((p) => ({
      label: p.label,
      value: p.seconds as number | "custom",
    })),
    { label: "Custom", value: "custom" },
  ]

  return (
    <div className="overflow-hidden rounded-lg border border-border bg-card">
      <div className="flex items-center justify-between gap-4 px-4 py-3">
        <div className="min-w-0">
          <p className="text-[13px] font-medium text-foreground">Auto-sync</p>
          <p className="text-[12px] text-muted-foreground">
            Check for new items on a schedule. Off &rarr; this source syncs only
            when you click Sync now.
          </p>
        </div>
        <Switch
          checked={enabled}
          onCheckedChange={onEnabledChange}
          aria-label="Auto-sync"
        />
      </div>

      {enabled && (
        <div className="space-y-3 border-t border-border px-4 py-3">
          <p className="text-[12px] font-medium text-muted-foreground">
            Frequency
          </p>
          <Segmented
            ariaLabel="Sync frequency"
            value={custom ? "custom" : intervalSeconds}
            options={freqOptions}
            onChange={(v) => {
              if (v === "custom") {
                const d = decompose(intervalSeconds)
                setValue(String(d.value))
                setUnit(d.unit)
                setCustom(true)
              } else {
                setCustom(false)
                onIntervalChange(v)
              }
            }}
          />

          {custom && (
            <div className="flex flex-wrap items-center gap-2 pt-0.5">
              <span className="text-[13px] text-muted-foreground">Every</span>
              <Input
                type="number"
                inputSize="sm"
                min={1}
                value={value}
                onChange={(e) => applyCustom(e.target.value, unit)}
                className="w-16"
                aria-label="Interval value"
              />
              <Segmented
                ariaLabel="Interval unit"
                value={unit}
                options={UNITS.map((u) => ({
                  label: u.label,
                  value: u.seconds,
                }))}
                onChange={(u) => applyCustom(value, u)}
              />
              <span
                className={cn(
                  "text-[12px]",
                  belowMin ? "text-destructive" : "text-muted-foreground"
                )}
              >
                Minimum 1 minute.
              </span>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
