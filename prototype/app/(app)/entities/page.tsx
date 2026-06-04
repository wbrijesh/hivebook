"use client"

import Link from "next/link"
import { RiAddLine, RiAlertLine } from "@remixicon/react"
import { toast } from "sonner"

import { ChromeActions } from "@/components/app/chrome"
import {
  HeadRow,
  Num,
  PageHeading,
  PageShell,
  Row,
  Section,
  StatBar,
  Tag,
  TableCard,
  Td,
  Th,
} from "@/components/app/page-kit"
import { Button } from "@/components/ui/button"
import { ENTITIES } from "@/lib/mock-book"
import { AMBIGUITY_ITEMS } from "@/lib/mock-admin"

export default function EntitiesPage() {
  const canonical = ENTITIES.filter((e) => e.status === "canonical").length
  const ambiguous = ENTITIES.filter((e) => e.status === "ambiguous").length
  const types = new Set(ENTITIES.map((e) => e.type)).size

  return (
    <>
      <ChromeActions>
        <Button size="sm" variant="outline" asChild>
          <Link href="/entities/ambiguity">
            <RiAlertLine className="size-3.5" />
            Ambiguity queue
            <Tag tone="warning">{AMBIGUITY_ITEMS.length}</Tag>
          </Link>
        </Button>
        <Button size="sm" onClick={() => toast("Add entity", { description: "Define a canonical name, type, and aliases." })}>
          <RiAddLine className="size-3.5" />
          Add entity
        </Button>
      </ChromeActions>

      <PageShell>
        <PageHeading
          title="Entities"
          description="The things the company cares about — resolved from the corpus and kept canonical across sources."
        />

        <StatBar
          items={[
            { label: "Entities", value: `${ENTITIES.length}`, sub: "canonical store" },
            { label: "Resolved", value: `${canonical}`, sub: "confident" },
            { label: "Ambiguous", value: `${ambiguous}`, sub: "need review" },
            { label: "Types", value: `${types}`, sub: "distinct" },
          ]}
        />

        <Section title="Entity store" count={ENTITIES.length}>
          <TableCard>
            <HeadRow>
              <Th className="min-w-0 flex-1">Entity</Th>
              <Th className="w-24">Type</Th>
              <Th className="w-24 text-right">Mentions</Th>
              <Th className="w-20 text-right">Sources</Th>
              <Th className="w-20 text-right">Aliases</Th>
              <Th className="w-24">Status</Th>
            </HeadRow>
            {ENTITIES.map((e) => (
              <Row key={e.id} href="#">
                <Td className="min-w-0 flex-1 truncate text-[13px] font-medium text-foreground">
                  {e.name}
                </Td>
                <Td className="w-24">
                  <Tag>{e.type}</Tag>
                </Td>
                <Num className="w-24">{e.mentions}</Num>
                <Num className="w-20">{e.sources}</Num>
                <Num className="w-20">{e.aliases}</Num>
                <Td className="w-24">
                  <Tag tone={e.status === "ambiguous" ? "warning" : "success"}>
                    {e.status === "ambiguous" ? "Ambiguous" : "Canonical"}
                  </Tag>
                </Td>
              </Row>
            ))}
          </TableCard>
        </Section>
      </PageShell>
    </>
  )
}
