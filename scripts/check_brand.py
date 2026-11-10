#!/usr/bin/env python3
"""Dependency-free palette, role, asset-integrity and text hygiene guard."""
import argparse
import fnmatch
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys
import xml.etree.ElementTree as ET

HEX = re.compile(r'(?<![\w#])#(?:[0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{4}|[0-9a-fA-F]{3})(?![\w])')
FUNCTION = re.compile(r'\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch|color)\s*\([^\n;]*?\)', re.I)
CSS_NAMED = re.compile(r'(?:\b(?:fill|stroke)\s*=\s*[\"\']|(?:color|background(?:-color)?|border-color|fill|stroke)\s*:\s*)([a-zA-Z]+)(?=[;\s\"\'}])')
# Files where a short hex colour is real syntax; elsewhere '#123' is usually an issue or PR number.
STYLE = {'.css', '.scss', '.html', '.svg', '.js', '.jsx', '.mjs', '.ts', '.tsx'}
TEXT = {'.py', '.mjs', '.js', '.jsx', '.ts', '.tsx', '.css', '.scss', '.html', '.svg', '.json', '.webmanifest', '.md', '.go', '.dart', '.yaml', '.yml', '.txt'}


def canonical(value):
    value = value.upper()
    if len(value) in (4, 5):
        value = '#' + ''.join(c * 2 for c in value[1:])
    return value


def check(root):
    errors = []
    token_path = root / 'brand/tokens.json'
    data = json.loads(token_path.read_text())
    colors = data['colors']
    for name, item in colors.items():
        if not re.fullmatch(r'#[0-9A-F]{6}', item['value']) or not item.get('role'):
            errors.append(f'token {name}: requires uppercase RGB hex and a role')
    allowed = {item['value'] for item in colors.values()}
    for surface, roles in data['surfaces'].items():
        for role, token in roles.items():
            if token not in colors:
                errors.append(f'{surface}.{role}: unknown token {token}')
    exceptions = data['unchecked']
    for item in exceptions:
        if not item.get('name') or not item.get('reason') or not item.get('paths'):
            errors.append('unchecked entries require name, paths and reason')
    # Git includes new, nonignored files and avoids caches and user data outside the repo.
    if (root / '.git').exists():
        raw = subprocess.check_output(['git', 'ls-files', '--cached', '--others', '--exclude-standard', '-z'], cwd=root)
        paths = sorted(set(raw.decode().split('\0')) - {''})
    else:  # Isolated mutation-test copies are not repositories.
        paths = [str(p.relative_to(root)) for p in root.rglob('*') if p.is_file()]
    for name in paths:
        path = root / name
        if not path.is_file() or (path.suffix not in TEXT and path.name != 'Makefile'):
            continue
        unchecked = [e for e in exceptions if any(fnmatch.fnmatchcase(name, pat) for pat in e['paths'])]
        if any(e.get('kind') != 'ansi' for e in unchecked):
            continue
        text = path.read_text(encoding='utf-8')
        if name != 'brand/tokens.json':
            for match in HEX.finditer(text):
                value = match.group()
                if len(value) in (4, 5) and value[1:].isdigit() and path.suffix not in STYLE:
                    continue  # issue/PR reference such as #123, not a colour
                if canonical(value) not in allowed:
                    errors.append(f'{name}: colour {match.group()} has no token')
            # Functional and named CSS colours must be expressed as tokens/hex.
            for match in FUNCTION.finditer(text):
                errors.append(f'{name}: functional colour {match.group()} has no token mapping')
            for match in CSS_NAMED.finditer(text):
                if match.group(1) not in {'none', 'currentColor', 'transparent', 'inherit', 'var', 'url'}:
                    errors.append(f'{name}: named colour {match.group(1)} has no token mapping')
        if name.startswith('brand/') and re.search(r'/(?:Users|home)/[^\s\"\']+', text):
            errors.append(f'{name}: personal absolute path')
    kit = root / 'brand/kit'
    expected = data['svg_fill_roles']
    actual_paths = {str(p.relative_to(root / 'brand')) for p in (kit / 'svg').glob('*.svg')}
    if actual_paths != set(expected):
        errors.append('SVG palette role coverage differs from the actual SVG files')
    for name, roles in expected.items():
        path = root / 'brand' / name
        if not path.exists():
            errors.append(f'{name}: missing SVG')
            continue
        doc = ET.parse(path)
        fills = [e.attrib['fill'] for e in doc.iter() if 'fill' in e.attrib]
        wanted = [colors[data['surfaces']['kit'][role]]['value'] for role in roles]
        if fills != wanted:
            errors.append(f'{name}: fill role drift: expected {wanted}, got {fills}')
    manifest_path = kit / 'icons/site.webmanifest'
    manifest = json.loads(manifest_path.read_text())
    for key, role in [('theme_color', 'compute'), ('background_color', 'background')]:
        wanted = colors[data['surfaces']['kit'][role]]['value']
        if manifest.get(key) != wanted:
            errors.append(f'site.webmanifest: {key} role drift')
    if manifest.get('name') != data['name'] or manifest.get('short_name') != data['name']:
        errors.append('site.webmanifest: product name drift')
    entries = json.loads((kit / 'manifest.json').read_text())['files']
    actual_files = {str(p.relative_to(kit)) for p in kit.rglob('*') if p.is_file()
                    and p.name != 'manifest.json'
                    and not any(part in {'.build', 'node_modules', '__pycache__', '.DS_Store'} for part in p.relative_to(kit).parts)}
    if actual_files != set(entries):
        errors.append('kit manifest file coverage differs from disk')
    for name, item in entries.items():
        path = kit / name
        if not path.is_file() or hashlib.sha256(path.read_bytes()).hexdigest() != item['sha256']:
            errors.append(f'kit/{name}: manifest hash drift; rebuild intentional changes')
    review = json.loads((kit / 'reference/pdf-review.json').read_text())
    if review['sha256'] != hashlib.sha256((kit / 'serverku-brand-guide.pdf').read_bytes()).hexdigest():
        errors.append('PDF manual review is stale')
    return errors


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, default=Path(__file__).resolve().parents[1])
    args = parser.parse_args()
    try:
        errors = check(args.root)
    except (KeyError, ValueError, OSError, ET.ParseError) as exc:
        errors = [f'invalid brand configuration: {exc}']
    if errors:
        print('\n'.join(errors), file=sys.stderr)
        return 1
    print('Brand guard passed: palette coverage, SVG roles, manifest and PDF review.')
    return 0


if __name__ == '__main__':
    sys.exit(main())
