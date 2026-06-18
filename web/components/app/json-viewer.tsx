"use client"

import { useState } from "react"
import { RiArrowRightSLine } from "@remixicon/react"

import { cn } from "@/lib/utils"

// JsonViewer renders parsed JSON as a restrained, collapsible tree. Apex: quiet
// monochrome, accent reserved — structure reads through weight and indentation,
// not colour. Objects/arrays past the auto-open depth start collapsed so large
// payloads (a GitHub issue, a project board) stay scannable.
const AUTO_OPEN_DEPTH = 1
const INDENT = 14 // px per level

export function JsonViewer({ value }: { value: unknown }) {
  return (
    <div className="overflow-auto font-mono text-[12.5px] leading-relaxed text-foreground">
      <JsonNode value={value} depth={0} />
    </div>
  )
}

function JsonNode({
  name,
  value,
  depth,
}: {
  name?: string
  value: unknown
  depth: number
}) {
  const isContainer = value !== null && typeof value === "object"
  const [open, setOpen] = useState(depth < AUTO_OPEN_DEPTH)
  const pad = { paddingLeft: depth * INDENT }

  if (!isContainer) {
    return (
      <div style={pad} className="py-0.5">
        {name !== undefined && <Key name={name} />}
        <Primitive value={value} />
      </div>
    )
  }

  const isArray = Array.isArray(value)
  const entries: [string, unknown][] = isArray
    ? (value as unknown[]).map((v, i) => [String(i), v])
    : Object.entries(value as Record<string, unknown>)
  const [openB, closeB] = isArray ? ["[", "]"] : ["{", "}"]
  const noun = isArray ? "items" : "keys"

  return (
    <div>
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        style={pad}
        className="flex w-full items-center gap-1 rounded py-0.5 text-left hover:bg-accent"
      >
        <RiArrowRightSLine
          className={cn(
            "size-3.5 shrink-0 text-muted-foreground transition-transform",
            open && "rotate-90"
          )}
        />
        {name !== undefined && <Key name={name} />}
        <span className="text-muted-foreground">
          {openB}
          {!open && (
            <>
              {" "}
              <span className="text-muted-foreground/70">
                {entries.length} {noun}
              </span>{" "}
              {closeB}
            </>
          )}
        </span>
      </button>
      {open && (
        <div>
          {entries.map(([k, v]) => (
            <JsonNode
              key={k}
              name={isArray ? undefined : k}
              value={v}
              depth={depth + 1}
            />
          ))}
          <div style={pad} className="py-0.5 text-muted-foreground">
            {closeB}
          </div>
        </div>
      )}
    </div>
  )
}

function Key({ name }: { name: string }) {
  return (
    <span className="shrink-0">
      <span className="font-medium text-foreground">{name}</span>
      <span className="text-muted-foreground">: </span>
    </span>
  )
}

function Primitive({ value }: { value: unknown }) {
  if (typeof value === "string") {
    return (
      <span className="break-all text-foreground">&quot;{value}&quot;</span>
    )
  }
  // Numbers, booleans, null read as secondary — quieter than string content.
  return (
    <span className="text-muted-foreground">
      {value === null ? "null" : String(value)}
    </span>
  )
}
