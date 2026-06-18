"use client"

import NumberFlow, { type NumberFlowProps } from "@number-flow/react"

import { cn } from "@/lib/utils"

export type AnimatedNumberProps = Omit<NumberFlowProps, "value"> & {
  value: number
}

export function AnimatedNumber({
  value,
  className,
  willChange = true,
  ...props
}: AnimatedNumberProps) {
  return (
    <NumberFlow
      value={value}
      willChange={willChange}
      className={cn("tabular-nums", className)}
      {...props}
    />
  )
}
