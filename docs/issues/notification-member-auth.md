# notification member auth

problem: the gateway authenticates every `/v1/notifications/*` request but hands
routed handlers only the device id, under a private context key
(`internal/gateway/gateway.go:189-193`). the notifier therefore authenticates the
bearer again (`internal/notifier/http.go:16-20`) to learn the member's kind, which
gates the phone-only receiver route and the `receiver` status in its state.

impact: one extra fleet lock and bearer digest per notification request, a
`fleet.Service` in `notifier.Config` (`internal/notifier/observer.go:31`) that
exists only for this, and a second failure path (401 `Unauthenticated` when a
member is removed between the two checks).

resolved when: the gateway passes the admitted `fleet.Member` to handlers,
`notifier.ServeHTTP` has no `Authenticate` call, and `notifier.Config` drops
`Fleet`.
