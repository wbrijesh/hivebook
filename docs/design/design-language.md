---
status: draft
last-reviewed: 2026-05-29
---

# Apex — Design Language

> **Apex** is the Trenches design system. Every UI decision should be
> checkable against this document.

## North star

**Trenches should feel engineered, not marketed — a precise, quiet instrument
that veterans admire for its craft and founders trust with their company's
brain.**

It is software, not an "experience." It behaves like a well-made tool:
predictable, exact, and out of the way. The interface is the means; the corpus
is the point. Nothing performs for attention. Quality is communicated the way
good engineering communicates it — by being correct in every detail, not by
saying it's good.

## Who it's for, and the one move that serves both

Two first-class users who look different:

- **The veteran** (think OpenBSD): values correctness, restraint, no bloat,
  no hand-holding, no dark patterns, behavior that matches the label exactly.
  Admires craft.
- **The founder** (just-funded, design-literate): wants something that feels
  stable, premium, and serious enough to hold their company's knowledge — not
  a toy.

They share exactly one aesthetic: **restraint.** The veteran reads restraint +
correctness as *rigor and respect*; the founder reads the same restraint as
*premium and stable*. So we don't compromise between them — we serve both by
doing **less, more precisely.** That is the whole trick.

Two ways we'd break the spell:

- **Too friendly/playful** (emoji, "Oops!", confetti, mascots) → the veteran
  recoils, the founder sees a toy.
- **Too raw/brutalist** (terminal cosplay, cold, dated) → the founder feels
  shut out; it reads as costume, not craft.

Target: **refined minimalism with engineering rigor** — current enough to feel
alive, restrained enough to feel serious. Neighbors in spirit: Linear's
precision, Stripe's trustworthy calm, Raycast's keyboard-nativeness.

## Principles

1. **Subtract first.** The default answer to "should we add this?" is no.
   Decoration is suspect; every element earns its place or leaves.
2. **Correctness is the aesthetic.** The small things being exactly right —
   the email that follows you, the provider name that matches the button, code
   vs. link — *is* the design. It's what the veteran admires and the founder
   feels as stability.
3. **Quiet over friendly.** Confidence is shown by saying less, not by being
   warm. We respect the user's intelligence.
4. **Content is the hero.** Chrome recedes; the corpus and the user's data are
   what's bright. Color and motion are rationed.
5. **Predictable over delightful.** No surprises, no easter eggs, no
   performative motion. It behaves exactly as a competent user expects.
6. **Built for people who live in tools.** Keyboard-respecting, legible state,
   power affordances present but never required.
7. **One system, not many screens.** Decisions are made once, in tokens and
   shared primitives, so quality holds as the surface grows.

## Voice

**Terse, with a human pulse.** Exact and declarative; respects the reader.
Not a robot, not a friend.

- Sentence case. No exclamation marks. No emoji. No cute ("Oops", "Yay").
- State what is. Cut filler adjectives and marketing language.
- Errors are factual and actionable, never apologetic-cute.
- Address a competent adult. Don't over-explain the obvious.
- Prefer verbs and specifics over reassurance.

| Don't | Do |
| --- | --- |
| "Oops! Something went wrong 😕 Let's try again!" | "That code is incorrect. Request a new one or try again." |
| "Welcome back! 🎉 Great to see you." | "Sign in or create your account." |
| "We're sending a magic link your way…" | "Enter the 6-digit code we sent to you@company.com." |
| "Awesome, your workspace is all set up!" | "Your workspace is ready." |

## Visual system

Tokens live in `prototype/app/globals.css`. Never use raw Tailwind palette
classes; always the semantic tokens. (See the project memory on the color
system.)

### Color

- **Neutrals:** pure Adobe Spectrum gray (no hue tint).
- **Accent:** one hue, used sparingly — primary buttons, focus rings, links,
  selected state. Carries meaning; never decorative. (Currently orange; the
  accent is configurable while we settle it.)
- **Status:** `success` (green), `warning` (yellow — kept distinct from the
  accent), `destructive` (red). Each has a `*-subtle` background + foreground
  pair for badges and callouts.
- **Surfaces — two content planes + one frame plane, identical logic in light
  and dark:**
  - **Base plane** = page background **and** inputs (recessed; a field reads
    as the base showing through the card).
  - **Card plane** = cards **and** secondary/outline buttons (raised, bordered).
  - **Spine plane** = the dark system frame (the left icon rail). It is dark in
    *both* themes — the frame reads the same whether the app is light or dark,
    which is what makes the product feel like one instrument rather than a set
    of pages. Tokens: `--spine`, `--spine-foreground`, `--spine-muted`,
    `--spine-accent`, `--spine-border`. Icons sit muted, lift to foreground on
    hover, take the accent when active.
  - Accent is reserved for primary actions only.
  - Exact hex differs per theme (white is the ceiling in light), but the
    *relationships* are identical.

### Type scale

One scale, no half-pixels. (Current code still has eyeballed sizes — migrating
to this is owed work.)

| px | role |
| --- | --- |
| 12 | fine print, metadata, hints (muted) |
| 13 | secondary text, labels, dense UI |
| 14 | **base** — body, default control text |
| 16 | emphasis, large inputs |
| 18 | card / section titles |
| 24 | page titles (rare) |

**14px is the base size** — the default for body and controls; 12/13 step
down, 16/18/24 step up. Weights: 400 body, 500 labels, 600 titles and buttons. Headings left-aligned
on forms; centered only on terminal/confirmation cards.

### Space, radius, borders

- 4px base grid (Tailwind spacing). Calm density: dense where it serves power
  users, never cramped.
- Radius from `--radius` (0.45rem); one family of rounding.
- Borders are visible but quiet (`--border`); inputs a touch stronger
  (`--input`). Definition over shadow.

## System surfaces (the application, dense end)

The auth and onboarding funnels are intentionally large and linear. The
**application** is the opposite register: it must read as a *system* — software
that gathers, builds, and documents — not a documentation site that happens to
have a sidebar. The difference is almost entirely **whether the system shows its
state and its work.** A docs site shows titles and prose; a system shows the
counts, freshness, lineage, and health behind them. We surface that.

Rules for every app surface:

1. **Frame, don't header.** Global navigation is the **spine** (a dark icon
   rail of surfaces + workspace + search + profile), always present — not a
   marketing top bar. Below it: an optional **contextual panel** (e.g. the Book
   tree), a **dense context bar** (object-type icon · breadcrumb · page-specific
   actions), then the work area. Three columns of instrument, not one page.
2. **Every object wears its state.** A row is never just a title and a chevron.
   It carries the facts a competent user would want at a glance: status, counts
   (sources, citations, entities, children), coverage, freshness, role, owner —
   in aligned, `tabular-nums` columns. Numbers build the trust that the system
   is actually working at scale.
3. **Show the machinery.** Ingestion (sources → artifacts), the build/rebuild
   pipeline, retrieval, and entity resolution are first-class, visible state:
   source health and last-sync, "rebuilt 2 days ago / rebuild queued", lineage
   (built from N spans across M sources), the ambiguity queue. The system's work
   is on screen, not hidden behind a tidy summary.
4. **Multi-pane over single-column.** Reading and admin surfaces pair a content
   column with a **persistent inspector** (Sources / Entities / Lineage), not a
   transient drawer. Tabs for parallel views. The centered narrow doc-column is
   for funnels, not the app.
5. **Density is the dense end of the type scale.** 13px workhorse, 12px
   metadata, 11px column headers (uppercase, muted); `tabular-nums` for any
   figure; mono for IDs, counts, and timestamps. Tight rows. Page titles are
   15–19px here, not 24 — the data is the hero, not the heading.
6. **No silent emptiness.** A surface with nothing to show still states the
   system fact ("building", "0 sources — they appear as the summary is built"),
   never a blank panel.

This is restraint *applied to information*, not restraint *as the absence of
it*: we show what serves the operator and nothing that merely decorates. The
quiet still holds — it's just dense.

## Motion

Functional only — continuity and feedback, then gone. Never a flourish.

- Route transitions: a quick cross-fade with a few px of settle ("next step in
  the same task," not "a journey to another room"). ~180ms.
- Everything respects `prefers-reduced-motion` and short-circuits to instant.
- Interactive accents (e.g. the cursor-tracked background) stay below the
  threshold of conscious notice.

## Iconography

- Line icons (Remixicon), sparing. Brand/provider logos in full color, sized
  via the icon library, never restyled.
- Icons label or aid scanning; they don't decorate. A header doesn't need one.

## Interaction & accessibility (defaults, not afterthoughts)

These are baked into the primitives, not caught in review:

- Keyboard reachable; visible focus rings; logical focus order.
- Live regions announce async status (verifying, errors); errors carry
  `role="alert"`.
- Move focus after transitions settle, not mid-animation.
- AA contrast as a floor; buy margin on the smallest text.
- No dark patterns. State is legible. Things behave exactly as labeled.

## We are not

Chatty · cute · gradient-happy · animation-forward · gamified · "delightful"
for its own sake · a template · a toy.

## Open questions

- **Accent hue.** Configurable for now; needs a final commit.
- **Density ceiling.** How dense the corpus/admin surfaces go before "calm
  density" tips into cramped — decide against the first real book-browse screen.
