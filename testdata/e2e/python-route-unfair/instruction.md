Implement `select_route(primary_healthy, replica_healthy)` for Boolean inputs over all four input pairs.
When only the primary is healthy, return integer `1`.
When only the replica is healthy, return integer `2`.
When neither endpoint is healthy, return integer `0`.
When both endpoints are healthy, either integer `1` or integer `2` is permitted.
The function has no side effects.
