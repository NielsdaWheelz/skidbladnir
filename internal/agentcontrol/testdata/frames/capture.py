#!/usr/bin/env python3
"""Regenerate observations.jsonl: paint every authored frame into its own pane on an isolated tmux server, read
each row as ObservePane does (`capture-pane -p -e`, one row per command), split the rows into ObservePane's
regions, apply the frame's declared byte-cap drops, and store the frame's expectation beside it.

Run from this directory: `python3 capture.py`. It uses only the tmux socket `skid-frames` with `-f /dev/null`,
creates and kills its own server, and needs tmux 3.4 or newer. Expectations live in the authored headers:
codex `expect=<activity>/<interaction>/<notice>/<composer> reason=<reason> rules=<id@region>,...|none`, claude
`expect=<activity>/<interaction>/<notice> <composer> reason=<reason> rules=...`.
"""
import json
import os
import re
import subprocess
import sys
import time

sys.dont_write_bytecode = True
from paint import frames  # noqa: E402

HERE = os.path.dirname(os.path.abspath(__file__))
ENVIRONMENT = {key: value for key, value in os.environ.items() if key not in ('TMUX', 'TMUX_PANE', 'TMUX_TMPDIR')}


def tmux(*args):
    return subprocess.run(['tmux', '-L', 'skid-frames', '-f', '/dev/null', *args],
                          capture_output=True, text=True, env=ENVIRONMENT, check=True).stdout


def expectation(header):
    fields = dict(part.split('=', 1) for part in header.split() if '=' in part)
    claude = header.startswith('=== ')
    activity, interaction, notice = fields['expect'].split('/')[:3]
    if claude:
        composer = header.split('expect=', 1)[1].split()[1]
    else:
        composer = fields['expect'].split('/')[3]
    rules = [] if fields['rules'] == 'none' else fields['rules'].split(',')
    return {'activity': activity, 'interaction': interaction, 'notice': notice, 'composer': composer,
            'reason': fields['reason'], 'rules': rules}


def regions(frame, rows):
    """ObservePane's regions: the bottom 64 rows, and rows 0..min(h-65,191) above them, then declared drops."""
    height = frame['height']
    header = frame['header']
    bottom_first = max(0, height - 64)
    bottom = {'kind': 'bottom', 'firstRow': bottom_first, 'rows': rows[bottom_first:], 'clipped': False}
    drop = re.search(r'obs=bottom\.first=(\d+),clipped|\bbclip=(\d+)', header)
    if drop:
        first = int(drop.group(1) or drop.group(2))
        bottom = {'kind': 'bottom', 'firstRow': first, 'rows': rows[first:], 'clipped': True}
    if height <= 64:
        return [bottom]
    top_end = min(height - 64, 192)
    top = {'kind': 'top', 'firstRow': 0, 'rows': rows[:top_end], 'clipped': height - 64 > 192}
    cut = re.search(r'\btclip=(\d+)', header)
    if cut:
        top = {'kind': 'top', 'firstRow': 0, 'rows': rows[:int(cut.group(1))], 'clipped': True}
    return [top, bottom]


def main():
    tmux('new-session', '-d', '-s', 'keep', '-x', '20', '-y', '5', 'sleep 3600')
    observations = []
    try:
        for corpus in ('codex.txt', 'claude.txt'):
            for frame in frames(os.path.join(HERE, corpus)):
                command = f"python3 '{HERE}/paint.py' '{HERE}/{corpus}' '{frame['id']}'; sleep 3600"
                pane = tmux('new-session', '-d', '-P', '-F', '#{pane_id}', '-x', str(frame['width']),
                            '-y', str(frame['height']), 'sh', '-c', command).strip()
                time.sleep(0.5)
                size = tmux('display', '-p', '-t', pane, '#{pane_width}x#{pane_height} #{alternate_on}').split()
                assert size[0] == f"{frame['width']}x{frame['height']}", (frame['id'], size)
                rows = []
                for row in range(frame['height']):
                    text = tmux('capture-pane', '-p', '-e', '-t', pane, '-S', str(row), '-E', str(row))
                    rows.append(text[:-1] if text.endswith('\n') else text)
                tmux('kill-session', '-t', pane)
                observations.append({
                    'id': frame['id'], 'provider': frame['provider'].capitalize(), 'width': frame['width'],
                    'height': frame['height'], 'alternate': size[1] == '1', 'regions': regions(frame, rows),
                    'expect': expectation(frame['header']),
                })
    finally:
        tmux('kill-server')
    with open(os.path.join(HERE, 'observations.jsonl'), 'w', encoding='utf-8') as out:
        for observation in observations:
            out.write(json.dumps(observation, ensure_ascii=False) + '\n')
    print(f'{len(observations)} observations')


if __name__ == '__main__':
    main()
