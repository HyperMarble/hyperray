// Argument facts retain source identity, layout shape, pass mode, and obligations.
// They never infer registers or machine validity from an opaque compiler value.

use super::scalar;
use super::type_class;
use hyperray_rust::mir::ArgumentFact;
use rustc_public::abi::{ArgAbi, PassMode};

pub(super) fn from_abi(argument: &ArgAbi) -> ArgumentFact {
    let shape = argument.layout.shape();
    ArgumentFact {
        source_type_class: type_class::name(argument.ty),
        size_bits: shape.size.bits(),
        align_bytes: shape.abi_align as usize,
        pass_mode: pass_mode_name(&argument.mode),
        layout_abi: scalar::layout_name(&shape.abi),
        scalar_components: scalar::components(&shape.abi),
        unresolved: pass_mode_obligations(&argument.mode),
    }
}

fn pass_mode_name(mode: &PassMode) -> String {
    match mode {
        PassMode::Ignore => "ignore".to_string(),
        PassMode::Direct(_) => "direct".to_string(),
        PassMode::Pair(_, _) => "pair".to_string(),
        PassMode::Cast { .. } => "cast".to_string(),
        PassMode::Indirect { .. } => "indirect".to_string(),
    }
}

fn pass_mode_obligations(mode: &PassMode) -> Vec<String> {
    match mode {
        PassMode::Ignore => Vec::new(),
        PassMode::Direct(_) => opaque_obligation("direct"),
        PassMode::Pair(_, _) => opaque_obligation("pair"),
        PassMode::Cast { pad_i32, .. } => {
            vec![format!("cast lowering detail is opaque; pad_i32={pad_i32}")]
        }
        PassMode::Indirect { on_stack, .. } => vec![format!(
            "indirect lowering detail is opaque; on_stack={on_stack}"
        )],
    }
}

fn opaque_obligation(mode: &str) -> Vec<String> {
    vec![format!("{mode} lowering detail is opaque")]
}
