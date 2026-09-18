// Retain compiler body payloads with exact block and statement positions.

use super::digest::digest;
use super::payload;
use hyperray_rust::mir::{RawBody, RawPosition};

pub(super) fn raw(
    body: &rustc_public::mir::Body,
    instance_id: &str,
    phase: &str,
) -> Result<RawBody, String> {
    let payload = payload::body(body);
    let body_digest = digest(&payload)?;
    let mut positions = Vec::new();
    for (block, basic_block) in body.blocks.iter().enumerate() {
        for (statement, value) in basic_block.statements.iter().enumerate() {
            positions.push(position(
                block,
                Some(statement),
                payload::statement(value),
                instance_id,
                &body_digest,
                phase,
            ));
        }
        positions.push(position(
            block,
            None,
            payload::terminator(&basic_block.terminator),
            instance_id,
            &body_digest,
            phase,
        ));
    }
    Ok(RawBody {
        phase: phase.to_string(),
        body_digest,
        positions,
        payload,
    })
}

fn position(
    block: usize,
    statement: Option<usize>,
    payload: serde_json::Value,
    instance_id: &str,
    body_digest: &str,
    phase: &str,
) -> RawPosition {
    let suffix = statement.map_or_else(
        || "terminator".to_string(),
        |index| format!("statement:{index}"),
    );
    RawPosition {
        block,
        statement,
        operation_id: format!("{instance_id}:{phase}:{body_digest}:block:{block}:{suffix}"),
        payload,
    }
}
