# serverku brand reference

Approved on 2026-11-10: current server + power artwork, Deep Teal / Active Teal palette, lowercase Sora Semibold 600 wordmark, and the placed-image rules below. Suggested headlines remain optional. This repository contains a Go CLI and Markdown documentation, with no web, Flutter or desktop application frontend. Terminal appearance is unchanged.

Start with [the kit](../../brand/kit/README.md), [overview](../../brand/kit/serverku-overview.png), [guide](../../brand/kit/serverku-brand-guide.pdf), [canonical tokens](../../brand/tokens.json) and [project brand skill](../../.claude/skills/serverku-brand/SKILL.md). Follow [review rules](../review.md) before a PR.

## Artwork and file selection

The upper server chassis and transparent power cutout represent disposable compute. The separate base represents configured persistent storage. Preserve the gap and negative space. Scale uniformly; do not add shadows, gradients, status lights, provider logos or live-state recolouring.

| Job | File under `brand/kit/` |
| --- | --- |
| Full standalone mark | `svg/serverku-mark-primary.svg` |
| Small mark | `svg/serverku-mark-micro-ink.svg` or `-white.svg` |
| Name beside mark | `svg/serverku-horizontal-primary.svg` on light; `-white.svg` on dark |
| Name below mark | `svg/serverku-stacked-primary.svg` or `-white.svg` |
| Name alone | `svg/serverku-wordmark-ink.svg` or `-white.svg` |
| Strict single ink | `-black.svg`, `-ink.svg` or `-white.svg` variants |
| Raster preview | Matching filename in `png/`; export from the vector source |
| Publishing tile | `icons/serverku-app-dark-<size>.png` or `-light-<size>.png` |
| Browser publishing | `icons/favicon-32.png`, `apple-touch-icon.png`, ICOs and sample `site.webmanifest` |
| macOS publishing | `icons/serverku.icns` and `serverku.iconset/` |

Static Markdown uses the generated, outlined lockup SVGs with token-controlled fills. It cannot inherit a host site's CSS tint. Interactive frontends, if added, must tint vector paths from their own token mapping rather than import a fixed-colour raster mark. Baked publishing icons are the intentional exception. None of these candidates registers a launcher, favicon or browser app by itself.

## Placement rules

- External clear space is one quarter of the **visible mark height** on all sides. Export canvas padding is not a substitute.
- Placed full-mark images are at least 48 px wide; micro images at least 16 px, normally 16-32 px; horizontal lockup images at least 240 px wide.
- These are placed-artwork requirements, not global UI header minimums. Future UI chrome needs a separately documented, legibility-tested compact recipe. Use the micro mark where appropriate.
- Use primary/deep teal on light surfaces and white on dark. The brand mark is an identity, never a live power/status control.

README places a 320 px lockup and switches to the white artwork using `prefers-color-scheme`. Its surrounding space is explicit. Relative image paths work in GitHub's repository view. No application asset base path or CSS mask is involved.

## Palette and mappings

`brand/tokens.json` is the only maintained colour source. Every colour includes a role. `surfaces.kit` maps the builder's local names to tokens; `svg_fill_roles` binds each exported SVG's ordered fills to those local roles. The browser publishing manifest also has checked background/name bindings.

| Token | Value | Role |
| --- | --- | --- |
| `ink` | `#173D42` | Chassis, wordmark, dark publishing background |
| `accent` | `#0D9488` | Storage base / brand accent |
| `white` | `#FFFFFF` | Reverse artwork and text, light background |
| `black` | `#000000` | Strict one-ink reproduction |
| `mist` | `#F2F8F7` | Guide panel background |
| `muted` | `#536B6E` | Supporting guide text |
| `border` | `#DAE4E2` | Overview swatch border |

`make brand-check` needs Python 3.11+ and no third-party packages. It scans tracked and new nonignored text files with the extensions listed in `scripts/check_brand.py`, plus the Makefile. Unknown hex values fail everywhere. In stylesheet and markup files (CSS, SCSS, HTML, SVG, JS/TS) every form is checked, including short hex and functional (`rgb()`, `hsl()` …) and named (`color: red`) colours, which must be replaced with token-backed values. In other files (Go, Markdown, YAML, JSON, Python) only hex is checked, and short all-decimal forms such as `#123` or hex followed by `-` (`#add-a-provider`) are treated as issue references and anchors, so ordinary prose does not fail. It also checks SVG fill roles, icon manifest roles, kit file coverage/hashes, the guide's manual-review hash, and that `tokens.json` matches the hash recorded at the last kit build, so a token or role change without `make brand-build` fails even when the colour only appears in raster or PDF outputs. The manifest protects binary exports from unchecked changes; rebuilding and artwork verification remain separate checks. The builder clears its generated folders and files first, so an output it no longer makes cannot survive a rebuild.

Known limits: this is a palette/integrity guard, not a general CSS/Go/Dart parser or an accessibility audit. It does not infer semantics from arbitrary new code, scan ignored files or detect every possible colour syntax. New surfaces must extend the mappings and scanner for their actual syntax. Existing terminal library defaults remain external user-controlled conventions, not a graphical theme enforced by this guard.

Exceptions live only in the named `unchecked` list with reasons: historical proposal notes and deliberately invalid mutation inputs are excluded; the Makefile's ANSI cyan is a terminal convention while its literal colours are still scanned. Do not add broad directory exclusions to bypass drift.

The mutation suite proves that an altered value, a new unmapped colour and a changed white reverse-wordmark colour all fail. The last mutation uses an otherwise approved muted token, so membership in the palette alone cannot conceal role drift. Additional cases cover named/functional colours, short hex in stylesheets, prose and code that must pass (issue references, Markdown anchors, YAML `color:` keys, Go comments), new SVG coverage, token and role changes without a rebuild, non-UTF-8 files and personal paths in historical notes. It does not claim a web or Flutter dark-theme test exists.

## Typography and surface recipes

Sora Semibold **600** is the graphical product wordmark. The supplied SVGs contain outlines and need no font loading. Only `serverku` is the wordmark; future labels such as “Admin” stay in the UI face. Plus Jakarta Sans Regular 400 / Semibold 600 is limited to the guide and supporting graphical materials. Static TTFs and both SIL OFL licences are bundled in `brand/kit/source/fonts/`.

- **README / Markdown:** use the outlined SVG lockup, with meaningful alt text. Body text and command examples use the renderer's normal fonts. Do not try to load a webfont into GitHub Markdown.
- **Guide / overview:** the builder loads the bundled static font files directly and resolves colours and the displayed name through repo tokens. Change source, never pixels or PDF output directly.
- **CLI / interactive prompts:** retain the user's terminal font, terminal colours and the existing `huh` defaults. Do not add a graphical logo, Sora font requirement or palette override to terminal output.
- **Future web frontend:** load Sora 600 only for the wordmark unless a broader scope is explicitly adopted; map UI roles to canonical tokens. Resolve public assets through the framework's base-path mechanism and verify dev and production loading. A failed mask hides the mark, so verify the actual network request and provide a visible fallback.
- **Future Flutter frontend:** use a token-mapped tinted vector. If a variable font is chosen, pair `fontWeight` with `fontVariations: [FontVariation.weight(n)]`. The currently bundled fonts are static; no Flutter integration exists.

## White-label limits

There is no current white-label configuration or brand pack. `tokens.json.name` supplies the kit's rendered name and sample browser-manifest name. It does **not** rename executable commands, cloud resources, configuration paths, asset filenames or the application's identity. A white-label frontend must introduce its own explicit name/colour mapping and guard coverage; never scatter literal product names or colours through that frontend.

## How to change the brand

1. Edit `brand/tokens.json` for colours, local role mappings and the graphical name. Keep every colour's role meaningful. When adding a form, add its SVG fill-role contract.
2. Edit `brand/kit/source/geometry.py` for geometry, `build_kit.py` for lockup/layout, or `brand-information.template.md` for supporting copy. For a type change, replace licensed font inputs, update font paths and historical-input verification deliberately, and visually inspect lettering.
3. Install pinned dependencies as described in [the kit README](../../brand/kit/README.md), then run `make brand-build`. It reads `brand/tokens.json` and the kit source and writes kit outputs/manifest; no ZIP, app snapshot or network is needed after dependency installation.
4. If the PDF changes, render **all seven pages**, inspect each, and only then update `brand/kit/reference/pdf-review.json` with the PDF SHA-256 and review method. Rebuild once more to refresh its manifest entry.
5. Run `make brand-verify` and `make brand-rebuild`. Two isolated rebuilds must match all checked-in kit bytes, including the PDF and manifest. Verification does not modify kit files.
6. Run `make brand-goldens`, open the two images in `brand/goldens/after/`, and compare with `before/`. Retain baseline goldens; reject unrelated changes such as dates. Goldens are reviewed manually, not pixel-compared in CI; Poppler versions can affect rendering.

The kit's `reference/historical-*` files are explicitly historical snapshots. They document proposal-era decisions and inspected feature-branch behavior, not current rules or guarantees. `reference/repo-context.json` hashes the build input (`brand/tokens.json`) and lists the code the guide's lifecycle copy (page 7) was reviewed against. Editing README or Go code never requires a kit rebuild; when `down`/`destroy` behaviour changes, update the page 7 copy and `brand-information.template.md`, then rebuild.

Current `master` removes compute on `down` while retaining configured persistent storage. Its `destroy` deletes the tracked disk and local project configuration; do not promise cleanup of snapshots or old restored disks based on historical proposal notes. Retained storage/provider resources can still incur charges, and estimates are not bills.
