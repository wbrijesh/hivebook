import { RiGithubFill, RiGoogleFill, RiPlugLine } from "@remixicon/react"

// Brand glyph per connector id; generic plug for anything new. Returns concrete
// JSX (no component-in-a-variable) so it's react-compiler clean. Shared across the
// add-connector gallery, the sources list, and the manage page so they match.
export function ConnectorIcon({
  connectorId,
  className,
}: {
  connectorId: string
  className?: string
}) {
  switch (connectorId) {
    case "github":
    case "github_pat": // token-auth GitHub — same brand glyph as the App connector
      return <RiGithubFill className={className} />
    case "gdocs":
      return <RiGoogleFill className={className} />
    default:
      return <RiPlugLine className={className} />
  }
}
