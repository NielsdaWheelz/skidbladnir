#!/usr/bin/env bash
# Prepare dev-server to serve the skid site (same shape as atlas-kit's deploy/devbox/install.sh and nexus-web's
# deploy/web/install.sh). Release selection belongs to release.py; services start once a release and the tunnel
# token exist.
set -euo pipefail
umask 077

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
stage=""
phase="preflight"

die() { printf 'skid site install: %s\n' "$*" >&2; exit 1; }
cleanup() {
  local result=$?
  if [[ -n "$stage" ]]; then rm -rf -- "$stage"; fi
  if (( result != 0 )); then
    printf 'skid site install: failed during %s; correct that input and rerun installation\n' "$phase" >&2
  fi
  exit "$result"
}
trap cleanup EXIT

(( $# == 0 )) || die 'usage: sudo deployment/site/install.sh'
(( EUID == 0 )) || die 'host installation requires root'
[[ "$(uname -s)" == Linux && "$(uname -m)" == x86_64 ]] || die 'host must be linux/x64'
for executable in curl sha256sum python3 systemctl groupadd useradd getent nologin cmp ss git; do
  command -v "$executable" >/dev/null || die "install required host command: $executable"
done
python3 -c 'import sys; sys.exit(0 if sys.version_info >= (3, 12) else 1)' || die 'python 3.12 or newer is required'
for file in serve.py release.py pull.py skid-web.service skid-web-tunnel.service skid-web-pull.service skid-web-pull.timer; do
  [[ -f "$script_dir/$file" ]] || die "missing installer input: $script_dir/$file"
done
if ! systemctl is-active --quiet skid-web.service && ss -Hltn 'sport = :3490' | grep -q .; then
  die 'port 3490 is already in use on this host'
fi

phase="pinned tool download"
install -d -o root -g root -m 0755 /opt/skid-web
stage="$(mktemp -d /opt/skid-web/.install.XXXXXX)"
# Same pinned release and checksum as nexus-web's and atlas-kit's tunnels (cloudflare/cloudflared release asset).
curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 --max-time 300 \
  https://github.com/cloudflare/cloudflared/releases/download/2026.10.4/cloudflared-linux-amd64 -o "$stage/cloudflared"
printf '%s  %s\n' ae5e8add7884b0531d66bbca4570d1de491ff6da51e4ebe7eb160cdf241d9e42 "$stage/cloudflared" | sha256sum --check --status
chmod 0755 "$stage/cloudflared"
[[ "$("$stage/cloudflared" --version)" == 'cloudflared version 2026.10.4 '* ]] || die 'cloudflared distribution version differs'

phase="service accounts and directories"
for account in skid-web skid-web-tunnel; do
  if ! getent group "$account" >/dev/null; then groupadd --system "$account"; fi
  if ! getent passwd "$account" >/dev/null; then
    useradd --system --gid "$account" --no-create-home --home-dir /nonexistent \
      --shell "$(command -v nologin)" "$account"
  fi
  [[ "$(id -u "$account")" != 0 && "$(id -Gn "$account")" == "$account" ]] || die "$account must have only its own unprivileged group"
  account_shell="$(getent passwd "$account")"
  [[ "${account_shell##*:}" == */nologin ]] || die "$account must use a nologin shell"
done
install -d -o root -g root -m 0755 /opt/skid-web/control /srv/skid-web
install -d -o root -g skid-web -m 0750 /srv/skid-web/releases
install -d -o root -g root -m 0700 /srv/skid-web/incoming /var/lib/skid-web
# The tunnel user can traverse to its token; nobody else can list the directory.
install -d -o root -g root -m 0711 /etc/skid-web
if [[ -e /etc/skid-web/cloudflared-token ]]; then
  chown root:skid-web-tunnel /etc/skid-web/cloudflared-token
  chmod 0640 /etc/skid-web/cloudflared-token
fi

phase="tool and service installation"
if ! cmp --silent "$stage/cloudflared" /opt/skid-web/cloudflared; then
  chown root:root "$stage/cloudflared"
  mv -f "$stage/cloudflared" /opt/skid-web/cloudflared
fi
install -o root -g root -m 0644 "$script_dir/serve.py" "$script_dir/release.py" "$script_dir/pull.py" /opt/skid-web/control/
install -o root -g root -m 0644 "$script_dir/skid-web.service" "$script_dir/skid-web-tunnel.service" \
  "$script_dir/skid-web-pull.service" "$script_dir/skid-web-pull.timer" /etc/systemd/system/
systemctl daemon-reload
if systemctl is-active --quiet skid-web.service; then systemctl restart skid-web.service; fi
printf 'skid site install: host prepared.\n'
