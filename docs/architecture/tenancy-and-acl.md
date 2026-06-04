---
status: living
last-reviewed: 2026-05-24
---

# Tenancy and Access Control

Trenches enforces access at two levels: across customers (strict
isolation), and within a customer (admin-managed, chapter-level).

## Across customers: strict isolation

- Every artifact, summary, embedding, entity, and audit record is
  tagged with a `tenant_id`.
- Every read path filters by `tenant_id` before any other filter.
- No cross-tenant learning, signal sharing, or shared book
  structure unless an explicit opt-in is in place. (No such opt-in
  exists in v0.1.)

## Within a customer: admin-managed chapter access

Trenches does **not** inherit source-system ACLs and does **not**
propagate source permissions through derived summaries. The reason
is operational: maintaining ACL propagation across heterogeneous
sources, through segmentation, entity resolution, and filing, into
derived summaries that aggregate across many artifacts, is a large
body of work and is fragile in practice. The admin's responsibility
is to configure access.

Access is granted **at the chapter level**. A user either has
access to a chapter or does not. Within a chapter the user sees
all topics and sub-topics.

### Default

By default, every member of the customer's organization has
visibility to every chapter in the book. This default reflects the
product thesis: the brain is a shared organizational asset.

### Admin chapter restrictions

Admins can narrow access for specific users or groups to a subset
of chapters. There is no per-topic or per-sub-topic access control.

### Access templates

To avoid per-user configuration at scale, admins can define
**access templates** representing personas, departments, or roles
(e.g., "Engineering", "Customer Support", "Finance"). A template
declares chapter access; applying a template to a user grants that
access. A user can have multiple templates applied (the resulting
access is the union).

Templates are created and applied **after some ingestion has
occurred**. At onboarding time the book has no chapters to
restrict, so the template authoring step waits until the book is
populated enough to be meaningful.

## Identity

User identity comes from the tenant's SSO provider via the auth
layer (see `enterprise-controls.md`). There is no per-source
identity mapping in Trenches — a consequence of not using source
ACLs.

The build-vs-buy decision for the auth provider itself (Zitadel,
WorkOS, or hand-built) is deferred to build time.

## Query-time enforcement

Every query path applies the requesting user's chapter access set
before returning results. Sub-topics, topics, and raw-artifact
spans outside the user's accessible chapters are filtered out
post-retrieval.

## Residency and data sovereignty

Region selection happens at the tenant level. Hosted tenants pick
a region cluster at onboarding; we operate Kubernetes clusters in
major regions (US, EU, Asia, expanding by demand). Self-hosted
tenants deploy on their own infrastructure in any region they
choose. See `enterprise-controls.md`.

## Audit

Every read and every privileged write is recorded to a per-tenant
audit log. See `enterprise-controls.md` for the audit log
specification.
