// Preserve callable instance ABI, generic arguments, body, and obligations.

use super::body::raw;
use super::kind::name;
use super::root_facts;
use hyperray_rust::mir::RawInstance;
use rustc_public::mir::mono::{Instance, InstanceKind};

pub(super) fn callable(
    instance: Instance,
    compilation_id: &str,
    symbol: String,
    target: &str,
) -> Result<RawInstance, String> {
    let id = format!("{compilation_id}:{symbol}");
    let phase = phase(instance.kind);
    let body = instance
        .body()
        .map(|value| raw(&value, &id, phase))
        .transpose()?;
    let (abi, abi_error, root_facts) = match instance.fn_abi() {
        Ok(value) => (
            Some(serde_json::json!({"debug": format!("{value:?}")})),
            None,
            Some(root_facts::from_abi(&instance, &value, target)),
        ),
        Err(error) => (None, Some(error.to_string()), None),
    };
    Ok(RawInstance {
        id,
        symbol,
        name: instance.name().to_string(),
        kind: name(instance.kind),
        generic_arguments: instance
            .args()
            .0
            .iter()
            .map(|argument| format!("{argument:?}"))
            .collect(),
        abi,
        abi_error,
        body_status: status(body.as_ref()),
        body,
        root_obligations: vec![
            "root reachability remains unresolved".to_string(),
            "runtime and input closure remain unresolved".to_string(),
        ],
        root_facts,
    })
}

fn phase(kind: InstanceKind) -> &'static str {
    match kind {
        InstanceKind::Shim => "shim",
        InstanceKind::Item
        | InstanceKind::Intrinsic
        | InstanceKind::LlvmIntrinsic
        | InstanceKind::Virtual { .. } => "optimized",
    }
}

fn status(body: Option<&hyperray_rust::mir::RawBody>) -> String {
    if body.is_some() {
        "present".to_string()
    } else {
        "absent".to_string()
    }
}
