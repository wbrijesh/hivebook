// Onboarding form options — real choices the user selects and we persist
// (design-doc 0005). Static for v0.1; may become server-driven later. Not mock.

export const companySizes = [
  { id: "1-10", label: "1 – 10" },
  { id: "11-50", label: "11 – 50" },
  { id: "51-200", label: "51 – 200" },
  { id: "201-500", label: "201 – 500" },
  { id: "501-2000", label: "501 – 2,000" },
  { id: "2001-5000", label: "2,001 – 5,000" },
  { id: "5000+", label: "5,000+" },
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

// code is an ISO alpha-2 country code for the region's flag ("EU" → European Union).
export const regions = [
  {
    id: "us-east",
    country: "United States",
    code: "US",
    city: "Washington DC",
    label: "us-east",
    recommended: true,
  },
  {
    id: "us-west",
    country: "United States",
    code: "US",
    city: "San Francisco",
    label: "us-west",
  },
  {
    id: "eu-central",
    // flagpack has no EU flag; Frankfurt is in Germany, so use the German flag.
    country: "European Union",
    code: "DE",
    city: "Frankfurt",
    label: "eu-central",
  },
  {
    id: "eu-west",
    country: "United Kingdom",
    code: "GBR",
    city: "London",
    label: "eu-west",
  },
  {
    id: "asia-south",
    country: "India",
    code: "IN",
    city: "Mumbai",
    label: "asia-south",
  },
  {
    id: "asia-east",
    country: "Japan",
    code: "JP",
    city: "Tokyo",
    label: "asia-east",
  },
  {
    id: "asia-southeast",
    country: "Singapore",
    code: "SG",
    city: "Singapore",
    label: "asia-southeast",
  },
  {
    id: "oceania",
    country: "Australia",
    code: "AU",
    city: "Sydney",
    label: "oceania",
  },
]
