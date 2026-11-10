#!/usr/bin/env python3
"""Rebuild twice in a temporary repository copy; require identical kit bytes."""
import hashlib
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

REPO = Path(__file__).resolve().parents[1]
IGNORED = {'node_modules', '.build', '__pycache__', '.DS_Store'}


def inventory(root):
    return {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in root.rglob('*') if p.is_file()
            and not any(part in IGNORED for part in p.relative_to(root).parts)}


def main():
    modules = REPO / 'brand/kit/source/node_modules'
    if not modules.exists():
        sys.exit('Run npm ci --prefix brand/kit/source first.')
    with tempfile.TemporaryDirectory(prefix='serverku-brand-rebuild-') as directory:
        copy = Path(directory) / 'repo'
        shutil.copytree(REPO, copy, ignore=shutil.ignore_patterns('.git', '.build', 'node_modules', '__pycache__', '.DS_Store', '*.zip'))
        # Only the native renderer dependency is shared; generated files remain isolated.
        (copy / 'brand/kit/source/node_modules').symlink_to(modules, target_is_directory=True)
        target = copy / 'brand/kit'
        expected = inventory(REPO / 'brand/kit')
        for run in (1, 2):
            subprocess.run([sys.executable, str(target / 'source/build_kit.py')], cwd=directory, check=True)
            actual = inventory(target)
            differences = [name for name in sorted(expected.keys() | actual.keys()) if expected.get(name) != actual.get(name)]
            if differences:
                sys.exit(f'Rebuild {run} differs: ' + ', '.join(differences))
    print('Two isolated rebuilds are byte-identical to the checked-in kit.')


if __name__ == '__main__':
    main()
