# directory listing hangs on a fifo token

problem: `POST /v1/directory-listings` with a token naming a fifo inside Home
(e.g. `{"directory":"~/fifo"}`) blocks in `open(2)` inside
`workdir.Service.List`'s `home.OpenRoot(relative)` until a writer opens the
fifo. there is no response and no log line, the request context cannot interrupt
it, and the handler goroutine leaks.

impact: low. listings never show fifos, so only a hand-made authenticated request
reaches it. each one pins a goroutine and a descriptor until a writer appears.

evidence: observed on darwin at 22fec70 and after the host-resources refactor,
both by a direct `List` call with a 1 s context (blocked past its deadline) and
over http (no response head within 2 s; the 422 `DirectoryListingUnavailable`
line appears once a writer opens the fifo). linux is untested.

repro: `mkfifo ~/fifo`, then list `~/fifo` from an authenticated client.

resolved when: listing a fifo, socket or other non-directory token answers 422
`DirectoryListingUnavailable` promptly on darwin and linux, with no goroutine
left blocked, and every other listing is byte-identical. candidates: reject a
non-directory with `home.Stat` before `OpenRoot` (a racing swap stays a
same-user hazard rules.md does not defend), or open without blocking. this is a
behaviour change and needs the owner's approval.
