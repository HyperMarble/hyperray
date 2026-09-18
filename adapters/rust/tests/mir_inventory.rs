// Raw inventory tests require every retained body position to stay observable.
// They do not accept the lossy assignment-only MIR projection.

use hyperray_rust::mir::{RawBody, RawPosition};
use serde_json::json;

#[test]
fn raw_body_keeps_statement_and_terminator_positions() {
    let body = RawBody {
        phase: "optimized".to_string(),
        body_digest: "digest".to_string(),
        positions: vec![
            RawPosition {
                block: 3,
                statement: Some(0),
                operation_id: "operation-statement".to_string(),
                payload: json!({"kind": "StorageLive"}),
            },
            RawPosition {
                block: 3,
                statement: None,
                operation_id: "operation-terminator".to_string(),
                payload: json!({"kind": "Return"}),
            },
        ],
        payload: json!({"blocks": [{"statements": [], "terminator": {"kind": "Return"}}]}),
    };
    assert_eq!(body.positions.len(), 2);
    assert_eq!(body.positions[0].statement, Some(0));
    assert_eq!(body.positions[1].statement, None);
    assert_eq!(body.positions[1].payload["kind"], "Return");
}
