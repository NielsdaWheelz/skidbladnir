# Publishing the site

The site at **https://skid.nielseriknandal.com** is the committed `site/` folder, served from **dev-server** the
way nexus-web's frontend and the atlas site are: a static server on loopback (`skid-web.service`, port 3490)
behind a named Cloudflare tunnel (`skid-web-tunnel.service`). Nothing is built on the server and no port is open:
the tunnel connects outwards and the server listens on loopback only.

**Pushing `main` publishes.** Every five minutes `skid-web-pull.timer` fetches `main` over https and, when the
tree of `site/` differs from the one last released, streams `git archive <sha> site` into `release.py install
<sha>`. That checks the archive (plain files under `site/` only, both pages present), unpacks it to
`/srv/skid-web/releases/<sha>/`, switches `/srv/skid-web/current` to it, deletes every other release and checks the
server serves it. No code from the repository runs on the server. Logs: `journalctl -u skid-web-pull`.

- Preview a change as the server will serve it: `python3 deployment/site/serve.py 8000 site`, then
  http://127.0.0.1:8000/.
- Roll back: revert the commit on `main` and push. The pull timer releases the revert within five minutes.
- After changing `serve.py`, `release.py`, `pull.py` or a unit, rerun the installer (below); it reinstalls them
  and restarts `skid-web` if it runs.

## Server setup (once)

On dev-server, as an administrator, from a copy of this folder:

1. `sudo deployment/site/install.sh`: pinned `cloudflared` (the same release and checksum as nexus-web and
   atlas) in `/opt/skid-web/`, service users `skid-web` and `skid-web-tunnel` (no login), `/srv/skid-web/`,
   `/var/lib/skid-web/`, `/etc/skid-web/` and the four units. It starts nothing.
2. Cloudflare: a named tunnel `skid-web` whose ingress is [tunnel-config.json](tunnel-config.json)
   (`skid.nielseriknandal.com` to `http://127.0.0.1:3490`, dot segments and anything else 404), and a proxied
   `CNAME skid` to `<tunnel id>.cfargotunnel.com`. Cloudflare owns the certificate.
3. The connector token goes in `/etc/skid-web/cloudflared-token`, owned `root:skid-web-tunnel`, mode `0640`
   (`sudo install -o root -g skid-web-tunnel -m 0640 /dev/stdin /etc/skid-web/cloudflared-token`). The token
   never goes in the repository, arguments or logs.
4. `sudo systemctl enable --now skid-web skid-web-tunnel`, then `sudo python3 -B /opt/skid-web/control/pull.py`
   for the first release, `sudo python3 /opt/skid-web/control/release.py check`, and
   `sudo systemctl enable --now skid-web-pull.timer`.

| file | role |
| --- | --- |
| `serve.py` | the static server: no listings or dotfiles, `/page` to `/page/`, `/install` to the latest release's installer (302), `/` as `llms.txt` to curl and HTTPie, pages revalidated, other files cached five minutes, one byte range per request for the video |
| `release.py` | install a release and check the server (root) |
| `pull.py`, `skid-web-pull.*` | the five-minute check of `main` that releases a changed `site/` |
| `install.sh`, `skid-web.service`, `skid-web-tunnel.service` | server setup |
| `tunnel-config.json` | the tunnel's ingress rules |

Adapted from atlas-kit's `deploy/devbox/` (its `PUBLISH.md`). The repository is public, so the pull needs no
deploy key.
