#!/usr/bin/env bash
# Put a commit's site/ live on dev-server, from any checkout that can `ssh dev-server-deploy` (like nexus-web's
# controller). Usage: deployment/site/release.sh [ref]   (default: origin/main, after fetching it)
set -euo pipefail

die() { printf 'skid site release: %s\n' "$*" >&2; exit 1; }
(( $# <= 1 )) || die 'usage: deployment/site/release.sh [ref]'
cd "$(git rev-parse --show-toplevel)"
git fetch --quiet origin
sha="$(git rev-parse --verify "${1:-origin/main}^{commit}")"
git cat-file -e "$sha:site/index.html" 2>/dev/null || die "$sha has no site/index.html"

ssh_opts=(-o BatchMode=yes -o ConnectTimeout=10 -o ServerAliveInterval=15 -o ServerAliveCountMax=4 -o StrictHostKeyChecking=yes)
printf 'skid site release: sending %s\n' "${sha:0:12}"
# shellcheck disable=SC2029 # $sha expands here on purpose: it is a verified 40-hex commit
git archive --format=tar "$sha" site | ssh "${ssh_opts[@]}" dev-server-deploy "sudo python3 -B /opt/skid-web/control/release.py install $sha"

for page in / /design/; do
  code="$(curl --silent --output /dev/null --write-out '%{http_code}' --max-time 20 "https://skid.nielseriknandal.com$page" || true)"
  printf 'skid site release: https://skid.nielseriknandal.com%s -> %s\n' "$page" "$code"
done
