# repeated inventory encoding inside the desktop client

problem: `internal/fleetclient/response.go` validates and discards a typed host
inventory. `client.go` decodes it again, serializes the projected inventory,
then decodes that result for filtering or target resolution. cli/tui consumers
decode it again. this mixes internal values with wire representation.

impact: repeated parsing and representation changes obscure ownership. runtime
cost has not been measured.

resolved when: existing typed results survive through fleetclient and its
consumers, with encoding at actual transport/output boundaries. preserve partial
peers, empty arrays, ordering, exact references, complete-inventory name
resolution, and uncertain mutation outcomes. characterize the real cli against
isolated gateways before changing the representation.

constraint: private `list` currently enforces the 1 mib unfiltered envelope
limit before filtering; the final envelope is checked again. moving that limit
changes behavior and requires an explicit decision. changing only private
`list` can add encoding work if public results remain raw json.
