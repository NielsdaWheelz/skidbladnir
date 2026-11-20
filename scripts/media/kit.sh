# shellcheck shell=bash disable=SC2034 # the scripts that source this use its variables
# Sourced by stage, record and cleanup: where the kit lives and works, and the
# owner's hosts.

kit=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
repo=$(cd "$kit/../.." && pwd -P)
work=${MEDIA_WORK:-$HOME/.cache/skid-media}
font=$HOME/Library/Fonts/skid-media-jetbrains-mono.ttf
groups=(atlas beacon cobalt)

# Fleet label, Go platform, and the command that runs a shell command there.
hosts="macbook darwin/arm64 local
dev-server linux/amd64 ssh -F $HOME/.ssh/config.d/dev-server dev-server
arch linux/amd64 ssh nnandal@arch"

fail() {
  printf '%s: %s\n' "${0##*/}" "$*" >&2
  exit 1
}

# on HOST COMMAND runs COMMAND in a shell on HOST, passing stdin through.
on() {
  local label reach
  local -a remote
  while read -r -u 3 label _ reach; do
    [[ $label == "$1" ]] || continue
    if [[ $reach == local ]]; then
      bash -c "$2"
    else
      read -ra remote <<<"$reach"
      "${remote[@]}" "$2"
    fi
    return
  done 3<<<"$hosts"
  fail "no host is labelled $1"
}
