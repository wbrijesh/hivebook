"use client"

import { type ReactNode, useState } from "react"
import Link from "next/link"
import { useParams, useRouter } from "next/navigation"
import { useMutation, useQuery } from "@connectrpc/connect-query"
import { toast } from "sonner"
import { RiArrowLeftLine, RiExternalLinkLine } from "@remixicon/react"

import { ConnectorIcon } from "@/components/app/connector-icon"
import { PageShell } from "@/components/app/page-kit"
import { Field } from "@/components/onboarding/onboarding-form"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { usePublishBreadcrumbLeaf } from "@/lib/breadcrumb-leaf"
import {
  connectWithToken,
  listConnectors,
} from "@/lib/gen/hivebook/integration/v1/integration-IntegrationService_connectquery"
import { listFeatureFlags } from "@/lib/gen/hivebook/tenant/v1/tenant-TenantService_connectquery"

// Per-connector setup guide for token auth — everything the user needs to create
// the right token, so they never have to guess at permissions.
const GUIDE: Record<
  string,
  {
    flag: string
    tokenUrl: string
    tokenUrlLabel: string
    placeholder: string
    steps: ReactNode[]
  }
> = {
  github_pat: {
    flag: "github_pat",
    tokenUrl: "https://github.com/settings/personal-access-tokens/new",
    tokenUrlLabel: "Open GitHub token settings",
    placeholder: "github_pat_…",
    steps: [
      <>
        Open GitHub →{" "}
        <b>
          Settings → Developer settings → Personal access tokens → Fine-grained
          tokens
        </b>{" "}
        (or use the button above), then <b>Generate new token</b>.
      </>,
      <>
        Give it a name and an expiration. Under <b>Resource owner</b>, pick the
        account or organization that owns the repositories you want to sync.
      </>,
      <>
        Under <b>Repository access</b>, choose <b>Only select repositories</b>{" "}
        and select the repos to sync.
      </>,
      <>
        Under <b>Repository permissions</b>, grant <b>read-only</b> access to{" "}
        <b>Contents</b>, <b>Issues</b>, and <b>Metadata</b> (Metadata is
        required). For project boards, also add <b>Projects: Read</b>.
      </>,
      <>
        Click <b>Generate token</b>, copy it, and paste it below. The token is
        stored encrypted and shown only once.
      </>,
    ],
  },
}

export default function ConnectTokenSourcePage() {
  const { connector } = useParams<{ connector: string }>()
  const router = useRouter()
  const connectors = useQuery(listConnectors, {})
  const flags = useQuery(listFeatureFlags, {})

  const guide = GUIDE[connector]
  const def = connectors.data?.connectors.find((c) => c.id === connector)
  const displayName = def?.name || "source"
  const enabled =
    !guide ||
    (flags.data?.flags.find((f) => f.key === guide.flag)?.enabled ?? false)

  usePublishBreadcrumbLeaf(def ? displayName : undefined)

  const [name, setName] = useState("")
  const [token, setToken] = useState("")
  const connect = useMutation(connectWithToken, {
    onSuccess: (res) => {
      // Hand back to the Sources list with a marker — the list shows the new source's
      // discovery progress and auto-opens its manage page once discovery finishes,
      // rather than parking the user on a connect page while the worker cold-starts.
      if (res.connection) router.push(`/sources?connected=${res.connection.id}`)
    },
    onError: (e) => toast.error(`Couldn't connect: ${e.message}`),
  })

  // Unknown / non-token connector, or the flag is off → don't expose the form.
  if (!guide || !enabled) {
    return (
      <PageShell>
        <Header displayName={displayName} connectorId={connector} />
        <p className="mt-6 text-[13px] text-muted-foreground">
          {!guide
            ? "This source can't be connected with a token."
            : "This source isn't enabled for this workspace. An admin can turn it on in Settings → Feature flags."}{" "}
          <Link href="/sources/new" className="underline">
            Back to sources
          </Link>
          .
        </p>
      </PageShell>
    )
  }

  function onConnect() {
    if (!name.trim() || !token.trim()) return
    connect.mutate({
      connectorId: connector,
      name: name.trim(),
      token: token.trim(),
    })
  }

  return (
    <PageShell>
      <Header displayName={displayName} connectorId={connector} />

      {/* Full width, stacked: the form, then the token guide. */}
      <div className="mt-8 space-y-10">
        <div className="space-y-5">
          <Field label="Source name" htmlFor="src-name">
            <Input
              id="src-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Work GitHub"
              className="h-9"
            />
          </Field>
          <Field label="Personal access token" htmlFor="src-token">
            <Input
              id="src-token"
              type="password"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") onConnect()
              }}
              placeholder={guide.placeholder}
              autoComplete="off"
              spellCheck={false}
              className="h-9"
            />
          </Field>
          <Button
            onClick={onConnect}
            disabled={!name.trim() || !token.trim() || connect.isPending}
          >
            {connect.isPending ? "Connecting…" : "Connect source"}
          </Button>
        </div>

        <div className="border-t border-border pt-6">
          <div className="flex items-baseline justify-between gap-3">
            <h2 className="text-[13px] font-semibold text-foreground">
              How to create a token
            </h2>
            <a
              href={guide.tokenUrl}
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-1 text-[12px] text-primary transition-opacity hover:opacity-80"
            >
              {guide.tokenUrlLabel}
              <RiExternalLinkLine className="size-3.5" />
            </a>
          </div>
          <ol className="mt-4 space-y-3">
            {guide.steps.map((step, i) => (
              <li
                key={i}
                className="flex gap-3 text-[13px] text-muted-foreground"
              >
                <span className="mt-px flex size-5 shrink-0 items-center justify-center rounded-full bg-muted text-[11px] font-medium text-foreground tabular-nums">
                  {i + 1}
                </span>
                <span className="leading-relaxed [&_b]:font-medium [&_b]:text-foreground">
                  {step}
                </span>
              </li>
            ))}
          </ol>
        </div>
      </div>
    </PageShell>
  )
}

function Header({
  displayName,
  connectorId,
}: {
  displayName: string
  connectorId: string
}) {
  return (
    <div className="flex items-center gap-3">
      <Button asChild variant="outline" size="icon-sm" aria-label="Back">
        <Link href="/sources/new">
          <RiArrowLeftLine />
        </Link>
      </Button>
      <ConnectorIcon
        connectorId={connectorId}
        className="size-6 shrink-0 text-foreground"
      />
      <div className="leading-tight">
        <h1 className="text-[15px] font-semibold tracking-tight text-foreground">
          Connect {displayName}
        </h1>
        <p className="text-[12px] text-muted-foreground">
          Authenticate with a personal access token.
        </p>
      </div>
    </div>
  )
}
