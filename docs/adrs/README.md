# Architecture Decision Records (ADRs)

An ADR is an immutable record of a single architectural decision:
the context that prompted it, the decision itself, the alternatives
considered, and the consequences.

## Why ADRs are separate from design docs

A design doc *proposes* — it can change, get rejected, evolve, and
be superseded. An ADR *records* — it captures a decision at a moment
in time. Both have value; collapsing them loses the audit trail of
"what did we decide, when, and why."

## When to write one

Whenever a decision:

- is hard or expensive to reverse
- affects how multiple components fit together
- locks in a vendor, format, or external dependency
- defines a contract that other parts of the system rely on

Decisions that are easily reversible (a function name, a library
version bump, an internal refactor) do not need ADRs.

## Immutability

Once an ADR is **accepted**, its body is not edited. If the decision
changes later, write a new ADR that **supersedes** the old one. The
old ADR stays in the record with its status updated to `superseded`
and a link to its replacement.

## Conventions

- Numbered `NNNN-kebab-case-title.md` starting at `0001-`.
- One decision per ADR. If you find yourself listing two decisions,
  write two ADRs.
- Use the frontmatter from `0000-template.md`.
- Keep them short — an ADR is not a design doc. The design doc
  explains the design; the ADR records the chosen point in the
  design space.
