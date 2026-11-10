"""Resolve the kit's local names through the repository's approved tokens."""
import json
from pathlib import Path

REPO = Path(__file__).resolve().parents[3]
TOKENS = json.loads((REPO / 'brand/tokens.json').read_text())
NAME = TOKENS['name']


def resolve(role):
    return TOKENS['colors'][TOKENS['surfaces']['kit'][role]]['value']


INK = resolve('compute')
TEAL = resolve('storage')
WHITE = resolve('reverse_mark')
BLACK = resolve('one_ink')
MIST = resolve('panel')
MUTED = resolve('supporting_text')
BORDER = resolve('swatch_border')

WORDMARK = resolve('wordmark')
DARK_TEXT = resolve('dark_text')
