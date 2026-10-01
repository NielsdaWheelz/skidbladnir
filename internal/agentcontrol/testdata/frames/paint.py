#!/usr/bin/env python3
"""Paint one authored frame to stdout: `paint.py codex.txt|claude.txt <frame-id>`, inside a pane of the frame's size.

Each row is written at its absolute position from the default style, after entering the alternate screen when the
frame says so. codex.txt tags are {name}; claude.txt tags are «name» (see each file's header for the notation).
"""
import re
import sys
import unicodedata

CODEX = {
    'b': '1', 'd': '2', 'i': '3', 'u': '4', 'r': '7', 'br': '1;7', 'bd': '1;2', '/': '0', '/fg': '39',
    'red': '38;5;1', 'grn': '38;5;2', 'yel': '38;5;3', 'mag': '38;5;5', 'cyan': '38;5;6',
    'acc': '38;2;99;168;248', 'run': '38;2;200;169;238', 'mdl': '38;2;246;226;183', 'dir': '38;2;171;223;167',
    'gly': '1;38;2;128;128;128', 'ora': '38;2;255;178;66', 'key': '38;2;64;64;64', 'sec': '38;2;115;115;115',
    'run2': '38;2;131;64;218', 'mdl2': '38;2;156;105;35', 'hl': '1;38;2;0;0;46;48;2;164;205;251',
    'pbg': '48;2;240;240;240', 'lgly': '1;38;2;32;32;32', 'lhdr': '38;2;141;141;141', 'lacc': '38;2;28;100;200',
    'warn': '38;2;196;167;103',
}
CLAUDE = {
    'f': '\x1b[38;2;153;153;153m', 'f2': '\x1b[38;2;215;119;87m', 'd': '\x1b[2m', 'i': '\x1b[7m', 'b': '\x1b[1m',
    'it': '\x1b[3m', 'g': '\x1b[48;2;55;55;55m', '/': '\x1b[0m', '/f': '\x1b[39m', '/g': '\x1b[49m', 'w': '\x1b[38;5;220m',
    'o': '\x1b]8;id=d1;https://example.invalid/dummy\x1b\\', '/o': '\x1b]8;;\x1b\\',
}
CLAUDE_TAG = re.compile(r'«([a-z0-9/]+)»')


def frames(path):
    """Every frame as {id, provider, width, height, alternate, header, rows: {row: source}}."""
    parsed = []
    for line in open(path, encoding='utf-8'):
        line = line.rstrip('\n')
        if line.startswith('#frame '):
            head = line.split()
            width, height = map(int, head[2].split('x'))
            fields = dict(part.split('=', 1) for part in head[3:] if '=' in part)
            parsed.append({'id': head[1], 'provider': 'codex', 'width': width, 'height': height,
                           'alternate': fields['alt'] == '1', 'header': line, 'rows': {}})
        elif line.startswith('=== '):
            head = line[4:].split()
            fields = dict(part.split('=', 1) for part in head[1:] if '=' in part)
            parsed.append({'id': head[0], 'provider': 'claude', 'width': int(fields['w']), 'height': int(fields['h']),
                           'alternate': fields['alt'] == '1', 'header': line, 'rows': {}})
        elif parsed and re.match(r'^\d{2,3}\|', line):
            row, text = line.split('|', 1)
            parsed[-1]['rows'][int(row)] = text
    return parsed


def codex_row(text):
    def tag(match):
        name = match.group(1)
        if name == 'fill':
            return '\x1b[K'
        if name.startswith('osc:'):
            return '\x1b]8;;' + name[4:] + '\x1b\\'
        if name == '/osc':
            return '\x1b]8;;\x1b\\'
        return '\x1b[' + CODEX[name] + 'm'
    return re.sub(r'\{([^{}]+)\}', tag, text)


def claude_width(text):
    return sum(0 if unicodedata.combining(c) else 2 if unicodedata.east_asian_width(c) in ('W', 'F') else 1
               for c in CLAUDE_TAG.sub('', text))


def claude_row(text, width):
    # X{n} repeats X; ⍽ is NBSP; ⟩ pads so the rest of the row ends at column width-2.
    text = re.sub(r'(.)\{(\d+)\}', lambda m: m.group(1) * int(m.group(2)), text).replace('⍽', ' ')
    if '⟩' in text:
        left, right = text.split('⟩', 1)
        text = left + ' ' * (width - 2 - claude_width(left) - claude_width(right)) + right
    return CLAUDE_TAG.sub(lambda m: CLAUDE[m.group(1)], text)


def main():
    frame = next(f for f in frames(sys.argv[1]) if f['id'] == sys.argv[2])
    out = ['\x1b[?1049h' if frame['alternate'] else '\x1b[?1049l', '\x1b[0m\x1b[H\x1b[2J']
    for row, text in sorted(frame['rows'].items()):
        painted = codex_row(text) if frame['provider'] == 'codex' else claude_row(text, frame['width'])
        out.append(f'\x1b[{row + 1};1H\x1b[0m' + painted + '\x1b[0m')
    out.append(f'\x1b[{frame["height"]};1H')
    sys.stdout.write(''.join(out))
    sys.stdout.flush()


if __name__ == '__main__':
    main()
