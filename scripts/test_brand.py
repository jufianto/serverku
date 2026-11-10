#!/usr/bin/env python3
"""Mutation-test the actual palette guard in disposable copies, never the checkout."""
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

REPO = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('brand_guard', REPO / 'scripts/check_brand.py')
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class PaletteMutations(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        shutil.copytree(REPO / 'brand', self.root / 'brand',
                        ignore=shutil.ignore_patterns('node_modules', '.build', '__pycache__', '*.zip', 'goldens'))

    def tearDown(self):
        self.temp.cleanup()

    def test_approved_baseline_passes(self):
        self.assertEqual(guard.check(self.root), [])

    def test_changed_colour_fails(self):
        path = self.root / 'brand/kit/svg/serverku-mark-primary.svg'
        path.write_text(path.read_text().replace('#173D42', '#173D43'))
        self.assertTrue(any('has no token' in error for error in guard.check(self.root)))

    def test_unmapped_new_colour_fails(self):
        (self.root / 'new-surface.css').write_text(':root { --new-accent: #123456; }\n')
        self.assertTrue(any('has no token' in error for error in guard.check(self.root)))

    def test_dark_reverse_wordmark_role_fails_even_with_an_approved_colour(self):
        # Reverse lockup lettering is the existing dark-background text surface.
        path = self.root / 'brand/kit/svg/serverku-horizontal-white.svg'
        path.write_text(path.read_text().replace('<g fill="#FFFFFF">', '<g fill="#536B6E">'))
        self.assertTrue(any('fill role drift' in error for error in guard.check(self.root)))

    def test_named_colour_fails(self):
        (self.root / 'new-surface.css').write_text('body { color: red; }\n')
        self.assertTrue(any('named colour' in error for error in guard.check(self.root)))

    def test_functional_colour_fails(self):
        (self.root / 'new-surface.css').write_text('body { color: rgb(1, 2, 3); }\n')
        self.assertTrue(any('functional colour' in error for error in guard.check(self.root)))

    def test_new_svg_requires_role_mapping(self):
        source = self.root / 'brand/kit/svg/serverku-mark-ink.svg'
        shutil.copy2(source, source.with_name('unmapped.svg'))
        self.assertTrue(any('role coverage' in error for error in guard.check(self.root)))

    def test_changed_token_requires_rebuild(self):
        path = self.root / 'brand/tokens.json'
        data = json.loads(path.read_text())
        data['colors']['ink']['value'] = '#173D43'
        path.write_text(json.dumps(data))
        self.assertTrue(guard.check(self.root))


if __name__ == '__main__':
    unittest.main()
