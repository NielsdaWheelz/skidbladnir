#!/usr/bin/env python3
"""Release whatever site/ is on main: fetch the repo and, when site/ has changed, install it (run as root).

Run by skid-web-pull.timer every five minutes, or by hand: sudo python3 -B /opt/skid-web/control/pull.py

It fetches only the newest commit of main into /var/lib/skid-web/repo.git over https (the repository is public,
so no key is needed), compares the tree of site/ with the one last released, and on a change streams
`git archive <sha> site` into release.py install, the same step deployment/site/release.sh runs over ssh. main
moves mostly for code, so an unchanged site/ releases nothing. No code from the repository runs on the server:
only the files under site/ are copied, after release.py has checked them.
Adapted from atlas-kit's deploy/devbox/pull.py.
"""
import os
import subprocess
import sys
from pathlib import Path

REPO_URL = "https://github.com/NielsdaWheelz/skidbladnir.git"
STATE = Path("/var/lib/skid-web")
MIRROR, RELEASED = STATE / "repo.git", STATE / "released-tree"
RELEASE = Path("/opt/skid-web/control/release.py")


def git(*args, **kw):
    env = {"PATH": "/usr/bin:/bin", "GIT_TERMINAL_PROMPT": "0", "HOME": str(STATE)}
    return subprocess.run(["git", f"--git-dir={MIRROR}", *args], env=env, check=True, **kw)


def main():
    if os.geteuid() != 0:
        sys.exit("skid site pull: run as root (sudo)")
    if not RELEASE.is_file():
        sys.exit(f"skid site pull: missing {RELEASE}; run deployment/site/install.sh")
    if not (MIRROR / "HEAD").is_file():
        STATE.mkdir(mode=0o700, exist_ok=True)
        subprocess.run(["git", "init", "--quiet", "--bare", str(MIRROR)], check=True)
    git("fetch", "--quiet", "--depth=1", "--no-tags", REPO_URL, "+refs/heads/main:refs/heads/main", timeout=300)
    sha = git("rev-parse", "refs/heads/main", capture_output=True, text=True).stdout.strip()
    tree = git("rev-parse", f"{sha}:site", capture_output=True, text=True).stdout.strip()
    if RELEASED.is_file() and RELEASED.read_text().strip() == tree:
        return
    print(f"skid site pull: site/ on main {sha[:12]} is tree {tree[:12]}; releasing")
    archive = subprocess.Popen(["git", f"--git-dir={MIRROR}", "archive", "--format=tar", sha, "site"], stdout=subprocess.PIPE)
    result = subprocess.run([sys.executable, "-B", str(RELEASE), "install", sha], stdin=archive.stdout)
    archive.stdout.close()
    if archive.wait() != 0 or result.returncode != 0:
        sys.exit(f"skid site pull: release of {sha[:12]} failed; the previous release stays live")
    RELEASED.write_text(tree + "\n")
    git("gc", "--quiet", "--prune=now", timeout=300)


if __name__ == "__main__":
    main()
