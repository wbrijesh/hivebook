// A right-pointing triangle used as the forward affordance on buttons — like a
// play icon, but stretched horizontally so it reads as motion/"go" rather than
// "play". Inherits the button's text color via currentColor; size via
// className (e.g. "size-4"). Only the size should ever change.
type Props = { className?: string }

export function PlayGlyph({ className }: Props) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      className={className}
      aria-hidden
      role="img"
    >
      <path
        d="M8 8 L17 12 L8 16 Z"
        fill="currentColor"
        stroke="currentColor"
        strokeWidth="1.1"
        strokeLinejoin="round"
        strokeLinecap="round"
      />
    </svg>
  )
}
