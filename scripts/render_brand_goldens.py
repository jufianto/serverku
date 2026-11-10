#!/usr/bin/env python3
"""Render maintained guide samples for manual before/after review (requires Poppler)."""
from pathlib import Path
import subprocess

REPO = Path(__file__).resolve().parents[1]
out = REPO / 'brand/goldens/after'
out.mkdir(parents=True, exist_ok=True)
for page in (1, 5):
    subprocess.run(['pdftoppm', '-f', str(page), '-l', str(page), '-singlefile',
                    '-scale-to', '1250', '-png',
                    str(REPO / 'brand/kit/serverku-brand-guide.pdf'),
                    str(out / f'guide-{page}')], check=True)
