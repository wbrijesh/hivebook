// Mock data for the admin surfaces (members, access, audit, usage, review,
// ambiguity). Kept consistent with the people and sources that appear in the
// Book's artifacts so the whole prototype reads as one workspace.

export type Role = "owner" | "admin" | "member" | "viewer"
export type MemberStatus = "active" | "invited" | "suspended"

export type Member = {
  id: string
  name: string
  email: string
  initial: string
  role: Role
  status: MemberStatus
  provisioned: "sso" | "manual"
  lastActive: string
}

export const MEMBERS: Member[] = [
  { id: "m-brijesh", name: "Brijesh Wawdhane", email: "google@brijesh.dev", initial: "B", role: "owner", status: "active", provisioned: "manual", lastActive: "now" },
  { id: "m-dana", name: "Dana Olsen", email: "dana@trenches-test.com", initial: "D", role: "admin", status: "active", provisioned: "sso", lastActive: "12 min ago" },
  { id: "m-priya", name: "Priya Raman", email: "priya@trenches-test.com", initial: "P", role: "admin", status: "active", provisioned: "sso", lastActive: "1 h ago" },
  { id: "m-sam", name: "Sam Okafor", email: "sam@trenches-test.com", initial: "S", role: "member", status: "active", provisioned: "sso", lastActive: "3 h ago" },
  { id: "m-lee", name: "Lee Carter", email: "lee@trenches-test.com", initial: "L", role: "member", status: "active", provisioned: "sso", lastActive: "yesterday" },
  { id: "m-maya", name: "Maya Lin", email: "maya@trenches-test.com", initial: "M", role: "member", status: "active", provisioned: "sso", lastActive: "2 days ago" },
  { id: "m-tomas", name: "Tomás Vidal", email: "tomas@trenches-test.com", initial: "T", role: "viewer", status: "active", provisioned: "sso", lastActive: "5 days ago" },
  { id: "m-aisha", name: "Aisha Khan", email: "aisha@trenches-test.com", initial: "A", role: "member", status: "invited", provisioned: "manual", lastActive: "—" },
  { id: "m-rob", name: "Rob Feeney", email: "rob@trenches-test.com", initial: "R", role: "member", status: "suspended", provisioned: "sso", lastActive: "3 weeks ago" },
]

export const ROLE_LABEL: Record<Role, string> = {
  owner: "Owner",
  admin: "Admin",
  member: "Member",
  viewer: "Viewer",
}

// — Access: chapter-level ACL via reusable templates —
export type AccessTemplate = {
  id: string
  name: string
  description: string
  /** Chapter ids granted, or "all". */
  chapters: string[] | "all"
  members: number
}

export const ACCESS_TEMPLATES: AccessTemplate[] = [
  { id: "at-everyone", name: "Everyone", description: "Default access for all members of the workspace.", chapters: ["ch-ops", "ch-eng", "ch-product"], members: 47 },
  { id: "at-leadership", name: "Leadership", description: "Full read across every chapter.", chapters: "all", members: 5 },
  { id: "at-engineering", name: "Engineering", description: "Engineering and Product knowledge.", chapters: ["ch-eng", "ch-product"], members: 18 },
  { id: "at-finance", name: "Finance", description: "Finance and operational policy, including restricted chapters.", chapters: ["ch-fin", "ch-ops"], members: 6 },
  { id: "at-support", name: "Support", description: "Customer-facing knowledge and operations.", chapters: ["ch-cust", "ch-ops"], members: 12 },
]

export type ChapterAccess = {
  chapterId: string
  scope: "all-members" | "restricted"
  templates: string[]
}

export const CHAPTER_ACCESS: ChapterAccess[] = [
  { chapterId: "ch-ops", scope: "all-members", templates: ["at-everyone", "at-finance", "at-support"] },
  { chapterId: "ch-eng", scope: "all-members", templates: ["at-everyone", "at-engineering"] },
  { chapterId: "ch-cust", scope: "all-members", templates: ["at-everyone", "at-support"] },
  { chapterId: "ch-fin", scope: "restricted", templates: ["at-leadership", "at-finance"] },
  { chapterId: "ch-people", scope: "restricted", templates: ["at-leadership"] },
  { chapterId: "ch-product", scope: "all-members", templates: ["at-everyone", "at-engineering"] },
]

// — Audit —
export type AuditKind = "read" | "write" | "admin" | "auth"
export type AuditEvent = {
  id: string
  time: string
  actor: string
  action: string
  target: string
  kind: AuditKind
  correlationId: string
}

export const AUDIT_EVENTS: AuditEvent[] = [
  { id: "e1", time: "14:38:02", actor: "Brijesh Wawdhane", action: "Signed in", target: "SSO · Okta", kind: "auth", correlationId: "req_8f2a41" },
  { id: "e2", time: "14:31:55", actor: "System", action: "Rebuilt summary", target: "EU VAT refunds over €500", kind: "write", correlationId: "job_b71c09" },
  { id: "e3", time: "14:22:10", actor: "Dana Olsen", action: "Viewed entry", target: "Sev-1 escalation path", kind: "read", correlationId: "req_4c9d22" },
  { id: "e4", time: "13:57:41", actor: "Priya Raman", action: "Updated access template", target: "Finance", kind: "admin", correlationId: "req_1a7e6b" },
  { id: "e5", time: "13:40:18", actor: "Sam Okafor", action: "Triggered sync", target: "Jira", kind: "admin", correlationId: "req_55f0aa" },
  { id: "e6", time: "12:18:03", actor: "System", action: "Resolved entity", target: "Finance → canonical", kind: "write", correlationId: "job_9d2244" },
  { id: "e7", time: "11:52:37", actor: "Tomás Vidal", action: "Ran ask", target: "\"refund threshold EU\"", kind: "read", correlationId: "req_77b310" },
  { id: "e8", time: "11:09:44", actor: "Brijesh Wawdhane", action: "Invited member", target: "aisha@trenches-test.com", kind: "admin", correlationId: "req_2e88c1" },
  { id: "e9", time: "10:47:21", actor: "Lee Carter", action: "Requested rebuild", target: "US refunds over $600", kind: "write", correlationId: "req_0bb934" },
  { id: "e10", time: "09:31:09", actor: "System", action: "Flagged ambiguity", target: "\"EM\" — 3 candidates", kind: "write", correlationId: "job_aa1f70" },
  { id: "e11", time: "09:02:55", actor: "Rob Feeney", action: "Sign-in blocked", target: "Account suspended", kind: "auth", correlationId: "req_6c4d80" },
  { id: "e12", time: "08:44:12", actor: "Maya Lin", action: "Viewed entry", target: "Refunds & adjustments", kind: "read", correlationId: "req_d109f3" },
]

// — Usage / metering —
export const USAGE = {
  plan: "Scale",
  period: "May 2026",
  seats: { used: 8, limit: 25 },
  tokens: { used: 4_240_000, limit: 10_000_000, query: 1_360_000, build: 2_880_000 },
  storage: { usedGb: 38.4, limitGb: 100 },
  syncs: { used: 14_820, limit: 50_000 },
}

// Last six months of token spend (millions) for a simple bar trend.
export const USAGE_TREND: { month: string; tokens: number }[] = [
  { month: "Dec", tokens: 2.1 },
  { month: "Jan", tokens: 2.6 },
  { month: "Feb", tokens: 3.0 },
  { month: "Mar", tokens: 3.9 },
  { month: "Apr", tokens: 3.7 },
  { month: "May", tokens: 4.24 },
]

// — Review queue: maintenance-loop proposals —
export type ProposalKind = "split" | "merge" | "reclassify" | "new-topic" | "unfiled"
export type ReviewProposal = {
  id: string
  kind: ProposalKind
  title: string
  detail: string
  target: string
  confidence: number
  raisedAt: string
}

export const REVIEW_PROPOSALS: ReviewProposal[] = [
  { id: "rp1", kind: "merge", title: "“Engineering Manager” and “EM”", detail: "Two entities resolve to the same role across 9 mentions.", target: "Entities", confidence: 0.91, raisedAt: "2 h ago" },
  { id: "rp2", kind: "split", title: "“Refunds & adjustments” by region", detail: "EU and US refund rules have diverged enough to warrant separate sub-topics.", target: "Operations / Refunds & adjustments", confidence: 0.82, raisedAt: "1 day ago" },
  { id: "rp3", kind: "new-topic", title: "Chargebacks", detail: "14 artifacts in #cs-refunds describe chargebacks with no home in the book.", target: "Operations", confidence: 0.74, raisedAt: "1 day ago" },
  { id: "rp4", kind: "reclassify", title: "“Sev-1 escalation path”", detail: "Increasingly referenced from Customer; consider cross-filing.", target: "Engineering / Incident response", confidence: 0.63, raisedAt: "3 days ago" },
  { id: "rp5", kind: "unfiled", title: "12 artifacts in #random", detail: "Low-signal channel; confirm whether to ingest or exclude.", target: "Slack · #random", confidence: 0.41, raisedAt: "4 days ago" },
]

// — Entity ambiguity queue —
export type AmbiguityCandidate = { name: string; score: number }
export type AmbiguityItem = {
  id: string
  surface: string
  container: string
  mentions: number
  raisedAt: string
  candidates: AmbiguityCandidate[]
}

export const AMBIGUITY_ITEMS: AmbiguityItem[] = [
  { id: "am1", surface: "EM", container: "Slack · #incidents", mentions: 9, raisedAt: "9 h ago", candidates: [{ name: "Engineering Manager", score: 0.62 }, { name: "Engagement Manager", score: 0.30 }, { name: "Email", score: 0.08 }] },
  { id: "am2", surface: "the console", container: "Notion · Finance", mentions: 7, raisedAt: "1 day ago", candidates: [{ name: "Billing console", score: 0.55 }, { name: "Admin console", score: 0.45 }] },
  { id: "am3", surface: "Falcon", container: "Jira · OPS", mentions: 5, raisedAt: "2 days ago", candidates: [{ name: "Project Falcon", score: 0.71 }, { name: "Falcon (vendor)", score: 0.29 }] },
  { id: "am4", surface: "Q4", container: "Notion · Policies", mentions: 4, raisedAt: "2 days ago", candidates: [{ name: "Q4 2025 audit", score: 0.66 }, { name: "Q4 OKRs", score: 0.34 }] },
]
