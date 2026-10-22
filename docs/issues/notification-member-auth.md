# notification member auth

problem: the gateway authenticates every `/v1/notifications/*` request but hands
the notifier only the device id, under a private context key. the notifier
therefore authenticates the bearer a second time to learn the member's kind, which
gates the phone-only receiver route and the `receiver` status in its state.

impact: one extra fleet lock and bearer digest per notification request, a
`fleet.Service` dependency in `notifier.Config` that exists only for this, and a
second failure path (401 `Unauthenticated` when a member is removed between the
two checks).

evidence: `internal/gateway/gateway.go:156` stores `member.DeviceID` under the
unexported `deviceContextKey{}`; `internal/notifier/http.go` begins `ServeHTTP`
with `fleet.Authenticate(r)`. `internal/gateway` belongs to the gateway-http
cleanup, so hub-attention left it unchanged.

resolved when: the gateway exposes the admitted `fleet.Member` to handlers and
`notifier.ServeHTTP` has no `Authenticate` call, so `notifier.Config` drops `Fleet`.
