# repeated inventory admission encoding

problem: `fleetclient.list` encodes the complete projected inventory to enforce
the size bound, then `Execute` repeats that encoding after a filter that can
only remove session rows. every browser inventory read pays both passes.

resolved when: return the admitted list directly after filtering. preserve the
first bound before filtering and name resolution, all other operations' bounds,
and final CLI encoding. characterize full/filtered/partial and near-limit
inventories through real HTTP, including oversized input that filtering would
otherwise hide. do not add encoded-byte caches or claim unmeasured latency gains.
