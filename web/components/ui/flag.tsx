import "react-flagpack/dist/style.css"
import FlagpackFlag, { type FlagProps } from "react-flagpack"

// House flag: real flag icons (flagpack) with our standard treatment — bordered,
// rounded, a subtle drop shadow, and a real-linear gradient. `code` is an ISO
// alpha-2 country code (e.g. "US"); "EU" renders the European Union flag. flagpack's
// `Flags` union is stricter than our region list needs, so we cast at this one
// boundary and let callers pass a plain string.
export function Flag({
  code,
  size = "l",
}: {
  code: string
  size?: "s" | "m" | "l"
}) {
  return (
    <FlagpackFlag
      code={code as FlagProps["code"]}
      size={size}
      hasBorder
      hasBorderRadius
      hasDropShadow
      gradient="real-linear"
    />
  )
}
