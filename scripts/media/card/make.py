"""Renders card.html, the 1280x640 social card, to $MEDIA_WORK/assets/social-card.png with headless Chrome,
then fails if it is over the 200 KB still limit.

usage: python3 scripts/media/card/make.py
"""
import os
import subprocess
import sys
import tempfile

CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
card = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'card.html')
out = os.path.join(os.environ.get('MEDIA_WORK') or os.path.expanduser('~/.cache/skid-media'), 'assets', 'social-card.png')
os.makedirs(os.path.dirname(out), exist_ok=True)
if os.path.exists(out):
    os.remove(out)
# A throwaway profile keeps the owner's Chrome profile out of it; file access lets the page load its fonts.
with tempfile.TemporaryDirectory(ignore_cleanup_errors=True) as profile:
    chrome = subprocess.run([CHROME, '--headless', f'--user-data-dir={profile}', '--allow-file-access-from-files',
                             '--hide-scrollbars', '--force-device-scale-factor=1', '--window-size=1280,640',
                             '--virtual-time-budget=5000', f'--screenshot={out}', 'file://' + card],
                            capture_output=True, text=True)
# Chrome's teardown can fail after it has written the screenshot, so judge by the file.
if not os.path.exists(out):
    sys.exit(f'make.py: Chrome wrote no screenshot (exit {chrome.returncode})\n{chrome.stderr[-2000:]}')
size = os.path.getsize(out)
print(f'social-card.png: {size / 1000:.0f} KB of 200 KB')
if size > 200_000:
    sys.exit('make.py: social-card.png is over the limit')
