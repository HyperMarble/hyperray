# Strict bounded behavior specification

## Persist payload

Scope: persist = the frozen row witnesses and frozen test inputs exercising persist.
Classify: persist = command bridges/persist_classify.sh.
Observe: persist."stored" = command bridges/observe_persist_stored.sh.
Inputs: persist(target: string, payload: string).
Grounding: persist.target_state."writable" = when target == "writable"; witness {"payload":"valid","target":"writable"}.
Grounding: persist.target_state."read-only" = when target == "read-only"; witness {"payload":"valid","target":"read-only"}.
Grounding: persist.payload_kind."valid" = when payload == "valid"; witness {"payload":"valid","target":"writable"}.
Grounding: persist.payload_kind."invalid" = when payload == "invalid"; witness {"payload":"invalid","target":"writable"}.

Parameters: `target_state` ("writable" / "read-only"), `payload_kind` ("valid" / "invalid").

| target_state | payload_kind | ID | Operation | Reachability | Required outcomes | Forbidden outcomes | Effects | Invariants | Input witnesses | Enforced by | Evidence | Constraint reason |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| "writable" | "valid" | REQ-persist-writable-valid | persist | reachable | return "stored" | raise ValidationError containing "invalid payload"; raise PermissionError containing "read-only target"; other outcome | write:target="stored" | none | [{"payload":"valid","target":"writable"}] | none | 1 | — |
| "writable" | "invalid" | REQ-persist-writable-invalid | persist | reachable | raise ValidationError containing "invalid payload" | return "stored"; raise PermissionError containing "read-only target"; other outcome | none | none | [{"payload":"invalid","target":"writable"}] | none | 1 | — |
| "read-only" | "valid" | REQ-persist-readonly-valid | persist | reachable | raise PermissionError containing "read-only target" | return "stored"; raise ValidationError containing "invalid payload"; other outcome | none | none | [{"payload":"valid","target":"read-only"}] | none | 1 | — |
| "read-only" | "invalid" | REQ-persist-readonly-invalid | persist | reachable | raise ValidationError containing "invalid payload" | return "stored"; raise PermissionError containing "read-only target"; other outcome | none | none | [{"payload":"invalid","target":"read-only"}] | none | 1 | — |

## Session request

Scope: session_request = the frozen row witnesses and frozen test inputs exercising session_request.
Classify: session_request = command bridges/session_request_classify.sh.
Observe: session_request."value" = command bridges/observe_session_request_value.sh.
Observe: session_request."written" = command bridges/observe_session_request_written.sh.
Inputs: session_request(session: string, request: string).
Grounding: session_request.session_state."active" = when session == "active"; witness {"request":"read","session":"active"}.
Grounding: session_request.session_state."closed" = when session == "closed"; witness {"request":"read","session":"closed"}.
Grounding: session_request.request_kind."read" = when request == "read"; witness {"request":"read","session":"active"}.
Grounding: session_request.request_kind."write" = when request == "write"; witness {"request":"write","session":"active"}.

Parameters: `session_state` ("active" / "closed"), `request_kind` ("read" / "write").

| session_state | request_kind | ID | Operation | Reachability | Required outcomes | Forbidden outcomes | Effects | Invariants | Input witnesses | Enforced by | Evidence | Constraint reason |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| "active" | "read" | REQ-session-active-read | session_request | reachable | return "value" | return "written"; raise StateError containing "closed session"; other outcome | read:session | none | [{"request":"read","session":"active"}] | none | 2 | — |
| "active" | "write" | REQ-session-active-write | session_request | reachable | return "written" | return "value"; raise StateError containing "closed session"; other outcome | write:session="updated" | none | [{"request":"write","session":"active"}] | none | 2 | — |
| "closed" | "read" | REQ-session-closed-read | session_request | reachable | raise StateError containing "closed session" | return "value"; return "written"; other outcome | none | none | [{"request":"read","session":"closed"}] | none | 2 | — |
| "closed" | "write" | CON-session-closed-write | session_request | excluded | — | — | — | — | none | none | — | the frozen API cannot issue write requests for a closed session |
