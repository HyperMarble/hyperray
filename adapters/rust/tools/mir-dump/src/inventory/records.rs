// Preserve static and global-assembly records without inventing bodies.

use hyperray_rust::mir::RawInstance;

pub(super) fn without_body(
    compilation_id: &str,
    symbol: String,
    name: String,
    kind: &str,
) -> RawInstance {
    RawInstance {
        id: format!("{compilation_id}:{symbol}"),
        symbol,
        name,
        kind: kind.to_string(),
        generic_arguments: Vec::new(),
        abi: None,
        abi_error: None,
        body_status: "absent".to_string(),
        body: None,
        root_obligations: vec!["root reachability remains unresolved".to_string()],
        root_facts: None,
    }
}
