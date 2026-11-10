# serverku brand kit

Working delivery as of 2026-11-10. Begin with `serverku-overview.png`, `brand-information.md` and `serverku-brand-guide.pdf`.

## Pick a file

- **Primary symbol:** `svg/serverku-mark-primary.svg`, deep-teal compute with active-teal storage. Its matching `png/` export is transparent.
- **One color:** `serverku-mark-ink`, `-black`, `-white` and `-teal`.
- **Small sizes:** `serverku-mark-micro-*`; wider power cutouts and one storage slot. Actual 16, 24, 32, 48 and 64px candidates are in `icons/serverku-symbol-*`.
- **Name + symbol:** `serverku-horizontal-*` and `serverku-stacked-*`. New proposed lowercase Sora Semibold wordmark; all lettering is outlined.
- **Name only:** `serverku-wordmark-*`.
- **Square publishing icons:** dark/light PNGs from 16 to 1024px, ICOs, dark ICNS and macOS iconset, favicon, Apple touch icon and sample relative-path web manifest.

Twenty original vector SVGs have matching PNGs. Marks, stacked lockups and app tiles are 2048px wide; horizontal lockups and wordmarks are 2400px wide. PNGs use RGBA; symbol and lettering cutouts are truly transparent. App tiles intentionally fill the rounded square and reveal that fill through their cutouts.

## Status and meaning

The owner selected server + power. This is **original vector design**, not a reconstruction of a supplied raster master. The upper chassis/power represents on-demand compute; the separate lower base represents configured persistent storage. All symbols use geometric paths with even-odd holes and have no masks, external fonts or embedded raster images.

Final shapes, palette, type and usage defaults are proposals. No existing app logo or palette was found in the inspected source. `brand-information.md` distinguishes product facts from new branding choices and explains that down deletes compute while configured storage remains; destroy removes storage too. Remaining provider charges may continue.

The delivery is directly in this repository under `brand/`. It does not change app logic, CLI output, credentials, cloud resources or releases. Publishing icon formats do not add GUI/browser/platform support.

## Rebuild

Requires Python 3.11+ and Node.js 20+ with npm:

```sh
python3 -m pip install -r source/requirements.txt
npm ci --prefix source
python3 source/build_kit.py
python3 source/verify_kit.py
```

The builder resolves bundled paths relative to itself and regenerates assets, reference sheets, PDF, manifest and the sibling ZIP. No app checkout or network is needed after dependencies are installed. Pinned dependencies and deterministic PDF/ZIP settings support repeatable builds.

After intentional PDF/artwork edits, render and inspect every guide page, then update `reference/pdf-review.json` to the new PDF SHA-256. The verifier rejects a stale manual-review record. `.build`, `node_modules`, Python caches and `.DS_Store` are excluded from the archive.

## Review evidence

`reference/` includes design rationale, high-zoom curves, actual/enlarged small-size checks, input provenance, brand tokens, manual PDF review and validation. `manifest.json` lists delivery file hashes and sizes. `source/geometry.py` is the editable symbol source; `source/build_kit.py` creates outlined lettering and all exports. Source fonts include OFL licenses.
