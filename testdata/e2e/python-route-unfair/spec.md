# Bounded route selection with permitted tie behavior

Inputs: select_route(primary_healthy: bool, replica_healthy: bool).
Grounding: select_route.primary_healthy."false" = when primary_healthy == false; witness {"primary_healthy":false,"replica_healthy":false}.
Grounding: select_route.primary_healthy."true" = when primary_healthy == true; witness {"primary_healthy":true,"replica_healthy":false}.
Grounding: select_route.replica_healthy."false" = when replica_healthy == false; witness {"primary_healthy":false,"replica_healthy":false}.
Grounding: select_route.replica_healthy."true" = when replica_healthy == true; witness {"primary_healthy":false,"replica_healthy":true}.

Parameters: `primary_healthy` ("false" / "true"), `replica_healthy` ("false" / "true").

| primary_healthy | replica_healthy | ID | Operation | Reachability | Required outcomes | Forbidden outcomes | Effects | Invariants | Input witnesses | Enforced by | Evidence | Constraint reason |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| "false" | "false" | no-route | select_route | reachable | return 0 | return 1; return 2; other outcome | none | none | [{"primary_healthy":false,"replica_healthy":false}] | test_non_tie_routes | 1;4 | — |
| "false" | "true" | replica-route | select_route | reachable | return 2 | return 0; return 1; other outcome | none | none | [{"primary_healthy":false,"replica_healthy":true}] | test_non_tie_routes | 1;3 | — |
| "true" | "false" | primary-only-route | select_route | reachable | return 1 | return 0; return 2; other outcome | none | none | [{"primary_healthy":true,"replica_healthy":false}] | test_non_tie_routes | 1;2 | — |
| "true" | "true" | both-routes-permitted | select_route | reachable | return 1; return 2 | return 0; other outcome | none | none | [{"primary_healthy":true,"replica_healthy":true}] | test_both_routes | 1;5 | — |
