#!/usr/bin/env python3
"""Install, activate and check releases of the skid site on dev-server. Runs as root (sudo).

  release.py install <sha>    read `git archive <sha> site` (a tar) from stdin, unpack it to
                              /srv/skid-web/releases/<sha>, check it and make it current
  release.py activate <sha>   make an installed release current again (rollback)
  release.py list             installed releases, newest first; * marks the current one
  release.py check            the site answers on loopback with the current release

deployment/site/pull.py (the pull timer) and deployment/site/release.sh (over ssh) drive this.
Activation swaps the /srv/skid-web/current link atomically; serve.py follows it on every request. The newest
five releases (and the current one) are kept. Adapted from atlas-kit's deploy/devbox/release.py.
"""
import fcntl
import grp
import os
import re
import shutil
import sys
import tarfile
import urllib.request
from pathlib import Path

SRV = Path("/srv/skid-web")
RELEASES, CURRENT, INCOMING = SRV / "releases", SRV / "current", SRV / "incoming"
PORT, KEEP, MAX_BYTES = 3490, 5, 64 * 1024 * 1024
PAGES = ("/", "/design/")


def die(msg):
    sys.exit(f"skid site release: {msg}")


def sha_arg(value):
    if not re.fullmatch(r"[0-9a-f]{40}", value):
        die("a release is named by its full 40-character commit sha")
    return value


def receive(sha):
    """stdin -> incoming/<sha>.tar, at most MAX_BYTES."""
    INCOMING.mkdir(mode=0o700, exist_ok=True)
    path, total = INCOMING / f"{sha}.tar", 0
    with open(path, "wb") as out:
        while chunk := sys.stdin.buffer.read(1 << 20):
            total += len(chunk)
            if total > MAX_BYTES:
                path.unlink()
                die(f"archive larger than {MAX_BYTES >> 20} MB")
            out.write(chunk)
    if not total:
        path.unlink()
        die("no archive on stdin; pipe `git archive --format=tar <sha> site` into this command")
    return path


def unpack(archive, sha):
    """Only directories and regular files under site/, no dot or parent segments; owned root:skid-web."""
    gid = grp.getgrnam("skid-web").gr_gid
    stage = RELEASES / f".{sha}.incoming"
    if stage.exists():
        shutil.rmtree(stage)
    stage.mkdir(mode=0o750)
    files = 0
    with tarfile.open(archive, mode="r:") as tar:
        for m in tar:
            parts = m.name.rstrip("/").split("/")
            if parts[0] != "site" or any(p in ("", ".", "..") or p.startswith(".") for p in parts):
                die(f"unexpected archive entry {m.name!r}")
            if len(parts) == 1:
                continue
            dest = stage.joinpath(*parts[1:])
            if m.isdir():
                dest.mkdir(mode=0o750, exist_ok=True)
            elif m.isfile():
                dest.parent.mkdir(mode=0o750, parents=True, exist_ok=True)
                with tar.extractfile(m) as src, open(dest, "xb") as out:
                    shutil.copyfileobj(src, out)
                os.chown(dest, 0, gid)
                os.chmod(dest, 0o640)
                files += 1
            else:
                die(f"archive entry {m.name!r} is not a plain file or directory")
    for d in [stage, *[p for p in stage.rglob("*") if p.is_dir()]]:
        os.chown(d, 0, gid)
        os.chmod(d, 0o750)
    for page in PAGES:
        if not (stage / page.strip("/") / "index.html").is_file():
            die(f"site{page} has no index.html")
    stage.rename(RELEASES / sha)
    print(f"skid site release: unpacked {files} files to {RELEASES / sha}")


def activate(sha):
    target = RELEASES / sha
    if not (target / "index.html").is_file():
        die(f"release {sha} is not installed")
    tmp = SRV / ".current.next"
    if tmp.is_symlink() or tmp.exists():
        tmp.unlink()
    tmp.symlink_to(target, target_is_directory=True)
    tmp.replace(CURRENT)
    print(f"skid site release: {sha[:12]} is current")


def current():
    return CURRENT.resolve().name if CURRENT.is_symlink() else None


def check():
    sha = current()
    if not sha:
        die("no current release")
    if os.system("systemctl is-active --quiet skid-web.service 2>/dev/null") != 0:
        die("skid-web.service is not running; start it: systemctl enable --now skid-web skid-web-tunnel")
    root = CURRENT.resolve()
    for page in PAGES:
        with urllib.request.urlopen(f"http://127.0.0.1:{PORT}{page}", timeout=15) as r:
            body = r.read()
        if r.status != 200 or body != (root / page.strip("/") / "index.html").read_bytes():
            die(f"{page} is not served from the current release")
    print(f"skid site release: {sha[:12]} serves {', '.join(PAGES)}")


def prune():
    keep = {current()}
    releases = sorted((p for p in RELEASES.iterdir() if p.is_dir() and not p.name.startswith(".")),
                      key=lambda p: p.stat().st_mtime, reverse=True)
    keep |= {p.name for p in releases[:KEEP]}
    for p in releases:
        if p.name not in keep:
            shutil.rmtree(p)


def main():
    if os.geteuid() != 0:
        die("run as root (sudo)")
    op, args = (sys.argv[1] if len(sys.argv) > 1 else ""), sys.argv[2:]
    lock = open(SRV / ".lock", "w")  # one release at a time: the pull timer and release.sh may meet
    fcntl.flock(lock, fcntl.LOCK_EX)
    if op == "install" and len(args) == 1:
        sha = sha_arg(args[0])
        if (RELEASES / sha).is_dir():
            sys.stdin.buffer.read()
            print(f"skid site release: {sha[:12]} already installed")
        else:
            archive = receive(sha)
            try:
                unpack(archive, sha)
            except BaseException:
                shutil.rmtree(RELEASES / f".{sha}.incoming", ignore_errors=True)
                raise
            finally:
                archive.unlink(missing_ok=True)
        activate(sha)
        prune()
        if os.system("systemctl is-active --quiet skid-web.service 2>/dev/null") == 0:
            check()
        else:
            print("skid site release: skid-web.service is not running yet; start it: systemctl enable --now skid-web skid-web-tunnel")
    elif op == "activate" and len(args) == 1:
        activate(sha_arg(args[0]))
        check()
    elif op == "list" and not args:
        cur = current()
        for p in sorted((p for p in RELEASES.iterdir() if p.is_dir() and not p.name.startswith(".")),
                        key=lambda p: p.stat().st_mtime, reverse=True):
            print(("* " if p.name == cur else "  ") + p.name)
    elif op == "check" and not args:
        check()
    else:
        die("usage: release.py install <sha> < archive.tar | activate <sha> | list | check")


if __name__ == "__main__":
    main()
