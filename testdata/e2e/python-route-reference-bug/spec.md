# Bounded route selection

Inputs: select_route(primary_healthy: bool, replica_healthy: bool).
Grounding: select_route.primary_healthy."false" = when primary_healthy == false; witness {"primary_healthy":false,"replica_healthy":false}.
Grounding: select_route.primary_healthy."true" = when primary_healthy == true; witness {"primary_healthy":true,"replica_healthy":false}.
Grounding: select_route.replica_healthy."false" = when replica_healthy == false; witness {"primary_healthy":false,"replica_healthy":false}.
Grounding: select_route.replica_healthy."true" = when replica_healthy == true; witness {"primary_healthy":false,"replica_healthy":true}.

Parameters: `primary_healthy` ("false" / "true"), `replica_healthy` ("false" / "true").

| primary_healthy | replica_healthy | ID | Operation | Reachability | Required outcomes | Forbidden outcomes | Effects | Invariants | Input witnesses | Enforced by | Evidence | Constraint reason |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| "false" | "false" | no-route | select_route | reachable | return 0 | return 1; return 2; other outcome | none | none | [{"primary_healthy":false,"replica_healthy":false}] | none | 1;4 | — |
| "false" | "true" | replica-route | select_route | reachable | return 2 | return 0; return 1; other outcome | none | none | [{"primary_healthy":false,"replica_healthy":true}] | test_replica_route | 1;3 | — |
| "true" | "false" | primary-only-route | select_route | reachable | return 1 | return 0; return 2; other outcome | none | none | [{"primary_healthy":true,"replica_healthy":false}] | test_primary_routes | 1;2 | — |
| "true" | "true" | primary-preferred-route | select_route | reachable | return 1 | return 0; return 2; other outcome | none | none | [{"primary_healthy":true,"replica_healthy":true}] | test_primary_routes | 1;2 | — |
