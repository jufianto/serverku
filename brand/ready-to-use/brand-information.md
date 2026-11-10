# serverku brand information

Working proposal as of 2026-11-10. Product facts follow the repository README and lifecycle command descriptions; visual choices are new proposals based on the owner-selected server + power direction.

## Name and positioning

**Name:** `serverku`, lowercase, matching the CLI and repository. Use code formatting for commands and plain lowercase for the product name.

**Short description:** An on-demand cloud VM orchestrator for Docker Compose projects.

**One sentence:** serverku creates cloud VMs for your Compose projects when you need them, and can retain configured persistent storage when the runtime is removed.

**Audience:** developers operating side projects, previews, staging environments, demos and internal tools that do not need compute running 24/7.

**Proposed headline:** Compute when you need it.

**Proposed supporting line:** Start your Compose project. Keep configured data between runs.

These lines are new suggestions, not historical approved taglines. Neither line is baked into the logo lockups.

## Product truths

- `serverku up <project>` creates a VM, attaches configured storage, provisions Docker, syncs the project and starts its Compose stack.
- `serverku down <project>` tears down the VM while retaining configured persistent storage. This is deletion of compute, not simply pausing a VM.
- `serverku destroy <project>` removes cloud resources, including persistent disks and snapshots. Do not promise retention after destroy.
- Only data placed on the configured persistent volume should be described as surviving down. Data written solely to ephemeral VM storage is different.
- Both GCP and DigitalOcean are implemented. Existing Compose projects, SSH/rsync, snapshots, optional DNS and Caddyku routing support the workflow.
- Retained storage and other provider resources may still incur charges. Price estimates are not bills; no fixed savings percentage or "zero cost while stopped" claim is justified.
- The software is early-stage. Branding should not imply guaranteed availability, enterprise certification, universal cloud support or automatic protection against data loss.

## Logo meaning

The **upper chassis and power control** represent disposable, on-demand compute. The **separate lower base** represents configured persistent storage. Their separation reinforces the lifecycle without needing a literal cloud, currency sign or provider logo.

The two-color primary uses deep teal for compute and active teal for storage. One-color versions preserve the same two-part structure. Color is not the only way to understand the mark.

The **micro variant** widens the power ring and stem and replaces the base's indicator/slot pair with one larger slot. It retains the chassis and storage gap. This is a deliberate small-size design, not a resized screenshot.

All delivered symbols are original hand-authored vector proposals using exact circular arcs and rounded rectangles. Negative space uses even-odd compound paths; there are no raster images, masks or tracing artifacts. No claim of similarity to an existing approved logo is made.

## Proposed visual system

| Token | Hex | Role |
|---|---|---|
| Deep Teal | `#173D42` | Compute chassis, lettering and main one-color ink |
| Active Teal | `#0D9488` | Persistent storage base and optional digital accent |
| White | `#FFFFFF` | Light background and reverse artwork |
| Black | `#000000` | Strict one-color reproduction |

These are proposed brand colors. No established application palette or fixed terminal theme is being replaced. The static identity must not be presented as a live power/status control.

**Wordmark:** Sora Semibold (600), lowercase, delivered as outlines.

**Supporting copy:** Plus Jakarta Sans Regular/Semibold, used in the PDF and optional future brand materials.

Both font families and their SIL Open Font License texts are bundled. These fonts do not change the user's terminal font or the CLI's output.

## Proposed usage defaults

- Primary symbol: start at 48px wide. Micro: use at 16-32px; inspect its actual appearance on the destination background.
- Horizontal lockup: start at 240px wide. Use the standalone symbol if lettering becomes too small.
- External clear space: one quarter of the visible symbol height on every side. Default canvas margins alone may not supply all the required space.
- Primary or deep teal on light surfaces; white on dark. Use black for strict one-ink production.
- Scale uniformly. Preserve the gap between compute and storage, power opening, rounded corners and transparent cutouts.
- Avoid gradients, glow, shadows, extra status lights, provider logos and live-state recoloring of the brand mark.

## Voice and claims

Be practical, calm and specific about resources and consequences. Prefer concrete workflow language over cloud jargon.

Use: "Start your Compose project when needed," "Remove compute while keeping configured storage," and "See current prices and clearly marked estimates."

Avoid: "serverless," "free when stopped," "never lose your data," "supports every cloud," or "save 90%" without evidence. Distinguish a destroyed VM from a stopped VM, retained data from backups, and price estimates from actual bills.

Example commands for brand materials:

```sh
serverku up demo
serverku status demo
serverku down demo
```

These are examples only; creating the kit does not execute them or provision cloud resources.

## Delivery and review

The complete kit lives under `brand/` in the Serverku repository. It includes SVGs, transparent PNGs, icon candidates, a seven-page PDF, this brand information, source context, editable geometry, licensed fonts, validation records and a portable ZIP.

The owner selected the conceptual direction. Final geometry, colors, lettering, headline and usage defaults await review. Platform publishing formats do not imply new GUI, browser or operating-system support. No app logic, credentials, cloud resources, release or deployment were changed.
