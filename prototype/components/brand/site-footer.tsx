import Link from "next/link"
import { cn } from "@/lib/utils"

const links = [
  { label: "Status", href: "#" },
  { label: "Docs", href: "#" },
  { label: "Help", href: "#" },
  { label: "Legal", href: "#" },
]

export function SiteFooter({ className }: { className?: string }) {
  return (
    <footer
      className={cn(
        "flex shrink-0 items-center justify-between border-t border-border bg-background px-6 py-5 text-[13px] text-muted-foreground",
        className
      )}
    >
      <div className="flex items-center gap-2">
        <span>Trenches © 2026</span>
      </div>
      <nav className="flex items-center gap-5">
        {links.map((l) => (
          <Link
            key={l.label}
            href={l.href}
            className="transition-colors hover:text-foreground"
          >
            {l.label}
          </Link>
        ))}
      </nav>
    </footer>
  )
}
