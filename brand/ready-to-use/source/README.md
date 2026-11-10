# Portable editable source

- `geometry.py`: original chassis, power and storage geometry using analytic arcs and rounded rectangles; full and micro versions.
- `build_kit.py`: SVG assembly, outlined Sora wordmarks, resvg raster exports, icons, review sheets, seven-page ReportLab PDF, manifest and deterministic ZIP.
- `render.mjs` and npm lockfile: pinned SVG renderer.
- `verify_kit.py`: structural checks plus current manual PDF-review record.
- `fonts/`: licensed Sora Semibold and Plus Jakarta Sans Regular/Semibold.
- `app-snapshots/`: read-only product context; no credentials or runtime configuration. The Go snapshot uses a `.go.txt` extension so repository-wide Go tooling does not treat it as a package.

Follow the parent README's commands. The entire build runs from this folder's bundled sources. `brand-information.md` and `reference/brand-tokens.json` should be kept in sync with intentional geometry, palette and naming changes.
