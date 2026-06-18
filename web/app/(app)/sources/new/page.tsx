"use client"

import { useMemo, useState } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { useMutation, useQuery } from "@connectrpc/connect-query"
import { toast } from "sonner"
import {
  RiArrowDownSLine,
  RiArrowLeftLine,
  RiSearchLine,
} from "@remixicon/react"

import { ConnectorIcon } from "@/components/app/connector-icon"
import { PageShell } from "@/components/app/page-kit"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import type { Connector } from "@/lib/gen/hivebook/integration/v1/integration_pb"
import {
  getAuthorizeUrl,
  listConnectors,
} from "@/lib/gen/hivebook/integration/v1/integration-IntegrationService_connectquery"
import { listFeatureFlags } from "@/lib/gen/hivebook/tenant/v1/tenant-TenantService_connectquery"

// Token-auth sources don't OAuth-redirect — they have a dedicated connect page
// (token + setup instructions) at /sources/new/<id>.
const TOKEN_CONNECTORS = new Set(["github_pat"])

// Add-source gallery: searchable, category-filterable grid of source types. Most
// authorize via a full-page OAuth redirect; token-auth sources (gated by a feature
// flag) route to their own connect page.
export default function AddSourcePage() {
  const router = useRouter()
  const connectors = useQuery(listConnectors, {})
  const flags = useQuery(listFeatureFlags, {})
  const authorize = useMutation(getAuthorizeUrl)

  const [query, setQuery] = useState("")
  const [category, setCategory] = useState("all")

  const patEnabled =
    flags.data?.flags.find((f) => f.key === "github_pat")?.enabled ?? false

  const all = useMemo(() => {
    const list = connectors.data?.connectors ?? []
    // Hide token-auth sources unless their feature flag is on.
    return list.filter((c) => c.id !== "github_pat" || patEnabled)
  }, [connectors.data, patEnabled])

  const categories = useMemo(
    () => ["all", ...Array.from(new Set(all.map((c) => c.archetype))).sort()],
    [all]
  )
  const shown = all.filter(
    (c) =>
      (category === "all" || c.archetype === category) &&
      c.name.toLowerCase().includes(query.trim().toLowerCase())
  )

  function connect(c: Connector) {
    if (TOKEN_CONNECTORS.has(c.id)) {
      router.push(`/sources/new/${c.id}`)
      return
    }
    authorize
      .mutateAsync({ connectorId: c.id })
      .then((res) => window.location.assign(res.url))
      .catch((e) => toast.error(`Couldn't start authorization: ${e.message}`))
  }

  return (
    <PageShell>
      <div className="flex items-start gap-3">
        <Button
          asChild
          variant="outline"
          size="icon-sm"
          aria-label="Back to sources"
        >
          <Link href="/sources">
            <RiArrowLeftLine />
          </Link>
        </Button>
        <div>
          <h1 className="text-[15px] font-semibold tracking-tight text-foreground">
            Add a source
          </h1>
          <p className="mt-0.5 text-[13px] text-muted-foreground">
            Connect a system to sync from. You can add more than one of the same
            type.
          </p>
        </div>
      </div>

      <div className="mt-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="relative w-full sm:max-w-xs">
          <RiSearchLine className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search sources"
            className="h-9 pl-8"
          />
        </div>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" size="sm" className="capitalize">
              {category === "all" ? "All categories" : category}
              <RiArrowDownSLine data-icon="inline-end" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-44">
            <DropdownMenuRadioGroup
              value={category}
              onValueChange={setCategory}
            >
              {categories.map((cat) => (
                <DropdownMenuRadioItem
                  key={cat}
                  value={cat}
                  className="capitalize"
                >
                  {cat === "all" ? "All categories" : cat}
                </DropdownMenuRadioItem>
              ))}
            </DropdownMenuRadioGroup>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {shown.map((c) => (
          <button
            key={c.id}
            onClick={() => connect(c)}
            disabled={authorize.isPending}
            className="flex items-center gap-3 rounded-lg border border-border bg-card px-4 py-4 text-left transition-colors hover:bg-accent disabled:opacity-60"
          >
            <ConnectorIcon
              connectorId={c.id}
              className="size-7 shrink-0 text-foreground"
            />
            <div className="min-w-0">
              <p className="text-[14px] font-medium text-foreground">
                {c.name}
              </p>
              <p className="text-[12px] text-muted-foreground capitalize">
                {c.archetype}
              </p>
            </div>
          </button>
        ))}
      </div>

      {connectors.isPending && (
        <p className="mt-6 text-[13px] text-muted-foreground">
          Loading sources…
        </p>
      )}
      {!connectors.isPending && shown.length === 0 && (
        <p className="mt-6 text-[13px] text-muted-foreground">
          No sources match.
        </p>
      )}
    </PageShell>
  )
}
