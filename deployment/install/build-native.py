#!/usr/bin/env python3
"""freeze the public helper, managed interpreter and hashed dependencies in a release."""
import hashlib
import json
import os
import pathlib
import shutil
import subprocess
import sys
import tarfile
import tempfile
import urllib.request

PINS = json.loads((pathlib.Path(__file__).resolve().parents[2] / "internal/install/native-runtime.json").read_text())
COMMIT, UV, PYTHON = (PINS[key] for key in ("commit", "uv", "python"))


def fetch(url, destination):
    with urllib.request.urlopen(url, timeout=120) as response:
        destination.write_bytes(response.read())


def run(*args, **kwargs):
    subprocess.run(args, check=True, **kwargs)


def main():
    platform, destination = sys.argv[1:]
    target = {"darwin-arm64": "aarch64-apple-darwin", "linux-amd64": "x86_64-unknown-linux-gnu"}[platform]
    python_target = {"darwin-arm64": f"cpython-{PYTHON}-macos-aarch64-none", "linux-amd64": f"cpython-{PYTHON}-linux-x86_64-gnu"}[platform]
    out = pathlib.Path(destination)
    out.mkdir()
    with tempfile.TemporaryDirectory(prefix="skid-native-build-") as scratch:
        scratch = pathlib.Path(scratch)
        source_tar = scratch / "source.tar.gz"
        fetch(f"https://codeload.github.com/NielsdaWheelz/llm-calling/tar.gz/{COMMIT}", source_tar)
        with tarfile.open(source_tar) as archive:
            archive.extractall(scratch, filter="data")
        source = scratch / f"llm-calling-{COMMIT}"
        shutil.copytree(source, out / "source", ignore=shutil.ignore_patterns("tests", ".git"))
        uv_tar = scratch / "uv.tar.gz"
        uv_asset = f"uv-{target}.tar.gz"
        fetch(f"https://github.com/astral-sh/uv/releases/download/{UV}/{uv_asset}", uv_tar)
        digest = scratch / "uv.sha256"
        fetch(f"https://github.com/astral-sh/uv/releases/download/{UV}/{uv_asset}.sha256", digest)
        if hashlib.sha256(uv_tar.read_bytes()).hexdigest() != digest.read_text().split()[0]:
            raise SystemExit("uv artifact checksum differs")
        with tarfile.open(uv_tar) as archive:
            archive.extractall(scratch / "uv", filter="data")
        shutil.copy2(scratch / "uv" / f"uv-{target}" / "uv", out / "uv")
        # the native build uv can fetch a foreign managed interpreter without executing it.
        builder = shutil.which("uv")
        if not builder or subprocess.check_output([builder, "--version"], text=True).split()[1] != UV:
            raise SystemExit(f"native helper build requires uv {UV}")
        environment = dict(os.environ, UV_PYTHON_INSTALL_DIR=str(out / "python"), UV_PYTHON_BIN_DIR=str(scratch / "bin"))
        run(builder, "python", "install", "--no-bin", python_target, env=environment)
        requirements = out / "requirements.txt"
        run(builder, "export", "--project", str(source), "--frozen", "--no-dev", "--extra", "claude-sdk", "--no-emit-project", "--no-header", "--no-annotate", "--output-file", str(requirements), stdout=subprocess.DEVNULL)
        run(builder, "pip", "install", "--target", str(out / "site-packages"), "--python-version", PYTHON,
            "--python-platform", target, "--only-binary", ":all:", "--require-hashes", "-r", str(requirements))
        # source is an immutable import root, rather than an editable checkout or runtime build.
        shutil.copytree(source / "src" / "provider_runtime", out / "site-packages" / "provider_runtime")
        # Native reads use only the SDK's session-file APIs. Its 259 MB bundled
        # provider executable is neither installed nor used by skid.
        shutil.rmtree(out / "site-packages/claude_agent_sdk/_bundled")
        for directory in out.rglob("__pycache__"):
            shutil.rmtree(directory)
        for path in out.rglob("*"):
            if path.is_symlink() and path.readlink().is_absolute():
                target_path = path.readlink()
                target_path.relative_to(out)
                path.unlink()
                path.symlink_to(os.path.relpath(target_path, path.parent))
        interpreter = next(path / "bin/python3.12" for path in (out / "python").iterdir()
                           if path.is_dir() and not path.is_symlink())
        manifest = {"commit": COMMIT, "uv": UV, "python": PYTHON, "interpreter": str(interpreter.relative_to(out)),
                    "lockSha256": hashlib.sha256((source / "uv.lock").read_bytes()).hexdigest(),
                    "requirementsSha256": hashlib.sha256(requirements.read_bytes()).hexdigest(), "excluded": ["claude_agent_sdk/_bundled"]}
        (out / "runtime.json").write_text(json.dumps(manifest, sort_keys=True) + "\n")
        # Dependency caches and wheels may carry private directory modes. The
        # complete release tree has canonical modes, independent of that cache
        # or the builder's umask; links retain their admitted relative targets.
        out.chmod(0o755)
        for path in out.rglob("*"):
            if path.is_symlink():
                continue
            if path.is_dir():
                path.chmod(0o755)
            elif path.is_file():
                path.chmod(0o755 if path.stat().st_mode & 0o111 else 0o644)


if __name__ == "__main__":
    main()
