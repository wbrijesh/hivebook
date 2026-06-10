import { PageShell, PageHeading } from "@/components/app/page-kit"

// Placeholder body for every app surface until it's built. The chrome is real;
// the content is intentionally a non-ideal "coming soon" state.
export function ComingSoon({ title }: { title: string }) {
  return (
    <PageShell>
      <PageHeading title={title} description="This surface isn't built yet." />
      <div className="mt-8 flex items-center justify-center rounded-md border border-dashed border-border bg-card/40 py-24">
        <div className="text-center">
          <p className="text-[13px] font-medium text-foreground">Coming soon</p>
          <p className="mt-1 text-[12px] text-muted-foreground">
            We&rsquo;re still building {title.toLowerCase()}.
          </p>
        </div>
      </div>
    </PageShell>
  )
}
