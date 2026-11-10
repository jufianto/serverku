# serverku maintained brand kit

Artwork, palette, Sora Semibold 600 lettering and placed-image rules were approved on 2026-11-10. Suggested headlines remain optional. Terminal appearance is unchanged.

See [the brand reference](../../docs/reference/brand.md) for file selection, usage, typography and changes. Start with the overview and seven-page guide in this folder.

## Files

- `svg/serverku-mark-primary.svg`: two-tone full mark; `-micro-*` for small uses.
- `svg/serverku-horizontal-*` and `serverku-stacked-*`: outlined name with mark.
- `svg/serverku-wordmark-*`: outlined product name only.
- `png/`: matching transparent previews; `icons/`: baked publishing icons, ICO/ICNS/iconset and sample relative-path browser manifest. These do not add a browser or desktop application.
- `source/`: maintained geometry, licensed static fonts and pinned Python/Node dependencies.
- `reference/historical-*`: historical proposal evidence, not current approval or runtime guarantees.
- `reference/repo-context.json`: hashes of current repository context read by the builder.

## Rebuild and verify

From the repository root, with Python 3.11+ and Node.js 20+:

```sh
python3 -m venv .venv && . .venv/bin/activate   # any venv outside brand/kit works
python3 -m pip install -r brand/kit/source/requirements.txt
npm ci --prefix brand/kit/source
make brand-build
make brand-check
make brand-verify
make brand-rebuild
```

The builder reads `brand/tokens.json` and current repository files. No application snapshots, ZIP or personal paths are needed. It writes the kit only. Dependencies are pinned; install them before the two isolated, network-free rebuild checks. The palette guard and its mutation tests use only Python's standard library.

After intentional PDF edits, render and inspect all seven pages before updating `reference/pdf-review.json` to the reviewed PDF SHA-256. Do not update the record just to silence a failure. `make brand-goldens` renders pages 1 and 5 for comparison; inspect these images too. Keep baseline goldens unchanged.

`manifest.json` hashes every kit file except itself. Verification is read-only. Rebuild after editing kit source or reference records to refresh the manifest.
