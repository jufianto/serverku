---
name: serverku-brand
description: Maintain serverku graphical branding, kit exports, palette mappings and documentation; use for logo, wordmark, identity or future frontend brand integration changes.
---

# serverku brand

Read [the brand reference](../../../docs/reference/brand.md) and [review rules](../../../docs/review.md). Canonical values and local-role/SVG contracts live in `brand/tokens.json`. The owner approved the current artwork, palette, Sora Semibold 600 wordmark and placed-image rules on 2026-11-10.

## Recipes

- Markdown: use the outlined horizontal SVG, primary on light and white on dark; provide alt text. The name is already outlined in Sora. Body copy uses the renderer's normal face.
- Guide/overview: edit `brand/kit/source/` and repo tokens, then rebuild. Fonts are bundled static TTFs with OFL licences; no webfont loading is needed.
- Terminal: preserve user-controlled font/colours and current prompt defaults. This project has no graphical app frontend or brand pack.
- Future frontend: map its semantic names to canonical tokens, supply the graphical name from an explicit brand configuration, tint vector geometry, and keep adjacent labels in the UI face. Verify asset loading under dev and production base paths; a failed CSS mask can hide the element. For Flutter variable fonts, specify both fontWeight and FontVariation.weight. Do not claim a frontend exists until it does.

Placed images use quarter-visible-mark-height clear space, 48 px full-mark / 16 px micro / 240 px horizontal minimum widths. These rules do not set global UI-header sizes. Keep artwork static across runtime status changes.

## Definition of done

- `make brand-check` passes, including the changed-value, new-colour and reverse-wordmark-role mutation checks. New surface syntax has explicit scanner/mapping coverage; exceptions are named with reasons in tokens.
- Run `make brand-build`, inspect every changed PDF page (all seven after guide changes), refresh the manual-review record only after inspection, and rebuild the manifest.
- `make brand-verify` and `make brand-rebuild` pass. Open regenerated after goldens and compare with preserved before goldens; keep dates stable.
- Brand reference, file-selection guidance and current code agree. Historical notes stay labelled as history. No copied app snapshots, personal paths, dependency trees or ZIPs enter the maintained kit.
- Run the applicable guards at each commit, follow repository review rules and obtain independent review before shipping. Preserve unrelated working-tree changes.
