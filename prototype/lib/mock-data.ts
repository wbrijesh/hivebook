// Shared mock data for the prototype. Keep this consistent across pages
// so navigation feels real — e.g., the user signed in on /sign-in should
// match the user shown in the onboarding top bar.

// How long simulated "backend" actions take (sign-in, sending a code,
// connecting a source, etc.). Single knob so we can speed up / slow down the
// whole prototype's loading states in one place.
export const MOCK_DELAY_MS = 3000

export const mockUser = {
  email: "google@brijesh.dev",
  name: "Brijesh Wawdhane",
  initial: "B",
  role: "Owner",
}

export const mockOrg = {
  name: "Hivebook Test Co",
  slug: "hivebook-test",
  domain: "hivebook-test.com",
  region: "us-east",
}

export const mockTenants = [
  { slug: "hivebook-test", name: "Hivebook Test Co", role: "Owner", members: 47 },
  { slug: "acme", name: "Acme Corp", role: "Member", members: 312 },
  { slug: "globex", name: "Globex Industries", role: "Admin", members: 1840 },
]

export const companySizes = [
  { id: "1-10", label: "1 – 10" },
  { id: "11-50", label: "11 – 50" },
  { id: "51-200", label: "51 – 200" },
  { id: "201-500", label: "201 – 500" },
  { id: "501-2000", label: "501 – 2,000" },
  { id: "2001-5000", label: "2,001 – 5,000" },
  { id: "5000+", label: "5,000+" },
]

export const departments = [
  "Engineering",
  "Product",
  "Design",
  "Customer Success",
  "Customer Support",
  "Sales",
  "Marketing",
  "Finance",
  "People & HR",
  "Legal & Compliance",
  "Security",
  "Operations",
  "Data",
  "IT",
]

export const useCases = [
  { id: "support", label: "Customer support knowledge" },
  { id: "engineering", label: "Engineering runbooks & incidents" },
  { id: "product", label: "Product decisions & specs" },
  { id: "sales", label: "Sales playbooks & objection handling" },
  { id: "operations", label: "Operational policies & SOPs" },
  { id: "people", label: "People onboarding & policies" },
  { id: "finance", label: "Finance approvals & exceptions" },
  { id: "security", label: "Security & incident response" },
  { id: "compliance", label: "Compliance & legal" },
  { id: "marketing", label: "Brand voice & marketing rules" },
  { id: "other", label: "Something else" },
]

export const dataTypes = [
  { id: "chat", label: "Chat & messaging", detail: "Slack, Microsoft Teams" },
  { id: "docs", label: "Documents & wikis", detail: "Notion, Confluence, Google Docs, SharePoint" },
  { id: "tickets", label: "Support tickets", detail: "Zendesk, Intercom, Front" },
  { id: "work", label: "Work tracking", detail: "Jira, Linear, GitHub Issues" },
  { id: "code", label: "Source code", detail: "GitHub, GitLab, Bitbucket" },
  { id: "meetings", label: "Meeting recordings", detail: "Gong, Granola, Fireflies" },
  { id: "crm", label: "Customer records", detail: "Salesforce, HubSpot" },
  { id: "observability", label: "Observability & incidents", detail: "PagerDuty, Sentry, Datadog" },
  { id: "email", label: "Email", detail: "Gmail, Outlook" },
  { id: "files", label: "File storage", detail: "Drive, Dropbox, OneDrive, Box" },
  { id: "warehouse", label: "Data warehouses", detail: "Snowflake, BigQuery, Redshift" },
  { id: "hr", label: "HR & people data", detail: "Workday, BambooHR, Rippling" },
]

export const regions = [
  { id: "us-east", country: "United States", city: "Washington DC", label: "us-east", flag: "🇺🇸", recommended: true },
  { id: "us-west", country: "United States", city: "San Francisco", label: "us-west", flag: "🇺🇸" },
  { id: "eu-central", country: "European Union", city: "Frankfurt", label: "eu-central", flag: "🇪🇺" },
  { id: "eu-west", country: "United Kingdom", city: "London", label: "eu-west", flag: "🇬🇧" },
  { id: "asia-south", country: "India", city: "Mumbai", label: "asia-south", flag: "🇮🇳" },
  { id: "asia-east", country: "Japan", city: "Tokyo", label: "asia-east", flag: "🇯🇵" },
  { id: "asia-southeast", country: "Singapore", city: "Singapore", label: "asia-southeast", flag: "🇸🇬" },
  { id: "oceania", country: "Australia", city: "Sydney", label: "oceania", flag: "🇦🇺" },
]

export const connectors = [
  { id: "slack", name: "Slack", category: "Chat", color: "#4A154B", letter: "S", popular: true },
  { id: "google-workspace", name: "Google Workspace", category: "Docs & email", color: "#4285F4", letter: "G", popular: true },
  { id: "github", name: "GitHub", category: "Code", color: "#181717", letter: "GH", popular: true },
  { id: "notion", name: "Notion", category: "Wiki", color: "#000000", letter: "N", popular: true },
  { id: "linear", name: "Linear", category: "Work tracking", color: "#5E6AD2", letter: "L", popular: true },
  { id: "jira", name: "Jira", category: "Work tracking", color: "#0052CC", letter: "J" },
  { id: "confluence", name: "Confluence", category: "Wiki", color: "#172B4D", letter: "C" },
  { id: "zendesk", name: "Zendesk", category: "Support", color: "#03363D", letter: "Z" },
  { id: "intercom", name: "Intercom", category: "Support", color: "#1F8DED", letter: "I" },
  { id: "salesforce", name: "Salesforce", category: "CRM", color: "#00A1E0", letter: "SF" },
  { id: "hubspot", name: "HubSpot", category: "CRM", color: "#FF7A59", letter: "H" },
  { id: "gong", name: "Gong", category: "Meetings", color: "#8039DF", letter: "GO" },
  { id: "granola", name: "Granola", category: "Meetings", color: "#FFB800", letter: "GR" },
  { id: "pagerduty", name: "PagerDuty", category: "Observability", color: "#06AC38", letter: "P" },
  { id: "sentry", name: "Sentry", category: "Observability", color: "#362D59", letter: "SE" },
  { id: "datadog", name: "Datadog", category: "Observability", color: "#632CA6", letter: "D" },
  { id: "snowflake", name: "Snowflake", category: "Data warehouse", color: "#29B5E8", letter: "SN" },
  { id: "drive", name: "Google Drive", category: "Files", color: "#0F9D58", letter: "D" },
  { id: "dropbox", name: "Dropbox", category: "Files", color: "#0061FF", letter: "DB" },
  { id: "workday", name: "Workday", category: "HR", color: "#F38B00", letter: "W" },
]

export const entityTypes = [
  { id: "product", label: "Products", placeholder: "e.g. Hivebook Cloud, Hivebook On-Prem" },
  { id: "service", label: "Services", placeholder: "e.g. Onboarding Service, Support Tier 2" },
  { id: "project", label: "Internal projects", placeholder: "e.g. Project Falcon, Migration H2" },
  { id: "tool", label: "Internal tools", placeholder: "e.g. Admin Console, Billing CLI" },
]
