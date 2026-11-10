# Maintained kit source

`geometry.py` defines the original server + power geometry. `palette.py` maps local roles through the repo's `brand/tokens.json`. `build_kit.py` outlines the product name from the bundled static Sora 600 font, renders exports, reads current repo context, builds the seven-page guide and writes its manifest. `brand-information.template.md` supplies maintained copy with token placeholders. `render.mjs` uses the pinned resvg renderer. `verify_kit.py` verifies without changing files.

Fonts and SIL OFL licences are in `fonts/`. There are no copied application snapshots. The builder is intentionally repository-based; it is not a standalone handoff archive. Follow the parent README for rebuild, review and verification commands.
