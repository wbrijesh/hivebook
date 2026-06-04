"use client"

import * as React from "react"
import { RiAddLine, RiLock2Line } from "@remixicon/react"
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
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { ACCESS_TEMPLATES, CHAPTER_ACCESS } from "@/lib/mock-admin"
import { getEntry } from "@/lib/mock-book"

const templateName = (id: string) =>
  ACCESS_TEMPLATES.find((t) => t.id === id)?.name ?? id

export default function AccessPage() {
  const [open, setOpen] = React.useState(false)
  const [name, setName] = React.useState("")
  const [description, setDescription] = React.useState("")
  const restricted = CHAPTER_ACCESS.filter((c) => c.scope === "restricted").length

  function create() {
    if (!name.trim()) return
    toast.success("Template created", { description: `${name} — grant chapters next.` })
    setOpen(false)
    setName("")
    setDescription("")
  }

  return (
    <>
      <ChromeActions>
        <Button size="sm" onClick={() => setOpen(true)}>
          <RiAddLine className="size-3.5" />
          New template
        </Button>
      </ChromeActions>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-[440px]">
          <DialogHeader>
            <DialogTitle>New access template</DialogTitle>
            <DialogDescription>
              A reusable persona you can grant chapters to and assign members.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3 py-1">
            <div className="space-y-1.5">
              <Label htmlFor="tpl-name">Name</Label>
              <Input
                id="tpl-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. Sales"
                autoFocus
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="tpl-desc">Description</Label>
              <Textarea
                id="tpl-desc"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="What this persona should be able to read."
                rows={3}
              />
            </div>
          </div>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">Cancel</Button>
            </DialogClose>
            <Button onClick={create} disabled={!name.trim()}>
              Create template
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <PageShell>
        <PageHeading
          title="Access"
          description="The book is shared by default. Chapters are the only ACL boundary; templates grant them to groups."
        />

        <StatBar
          items={[
            { label: "Chapters", value: `${CHAPTER_ACCESS.length}`, sub: "access-controlled" },
            { label: "Restricted", value: `${restricted}`, sub: "not all-members" },
            { label: "Templates", value: `${ACCESS_TEMPLATES.length}`, sub: "personas" },
            { label: "Default", value: "All", sub: "members can read" },
          ]}
        />

        <Section title="Chapters" count={CHAPTER_ACCESS.length}>
          <TableCard>
            <HeadRow>
              <Th className="min-w-0 flex-1">Chapter</Th>
              <Th className="w-32">Scope</Th>
              <Th className="min-w-0 flex-1">Granted via</Th>
            </HeadRow>
            {CHAPTER_ACCESS.map((c) => {
              const entry = getEntry(c.chapterId)
              const isRestricted = c.scope === "restricted"
              return (
                <Row key={c.chapterId} href="#">
                  <Td className="min-w-0 flex-1 gap-2 text-[13px] font-medium text-foreground">
                    {isRestricted && <RiLock2Line className="size-3.5 shrink-0 text-warning" />}
                    <span className="truncate">{entry?.title ?? c.chapterId}</span>
                  </Td>
                  <Td className="w-32">
                    <Tag tone={isRestricted ? "warning" : "muted"}>
                      {isRestricted ? "Restricted" : "All members"}
                    </Tag>
                  </Td>
                  <Td className="min-w-0 flex-1 flex-wrap gap-1">
                    {c.templates.map((t) => (
                      <Tag key={t}>{templateName(t)}</Tag>
                    ))}
                  </Td>
                </Row>
              )
            })}
          </TableCard>
        </Section>

        <Section title="Access templates" count={ACCESS_TEMPLATES.length}>
          <TableCard>
            <HeadRow>
              <Th className="w-40">Template</Th>
              <Th className="min-w-0 flex-1">Description</Th>
              <Th className="w-24 text-right">Chapters</Th>
              <Th className="w-24 text-right">Members</Th>
            </HeadRow>
            {ACCESS_TEMPLATES.map((t) => (
              <Row key={t.id} href="#">
                <Td className="w-40 text-[13px] font-medium text-foreground">{t.name}</Td>
                <Td className="min-w-0 flex-1 truncate text-[12px] text-muted-foreground">
                  {t.description}
                </Td>
                <Num className="w-24">{t.chapters === "all" ? "All" : t.chapters.length}</Num>
                <Num className="w-24">{t.members}</Num>
              </Row>
            ))}
          </TableCard>
        </Section>
      </PageShell>
    </>
  )
}
