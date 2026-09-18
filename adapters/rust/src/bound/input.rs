// Input shapes derived only from structural compiler types. The final GOTO
// model, not this early type view, defines the proof domain.

use super::{domain, Input};
use crate::mir;

pub fn analyze(values: &[mir::Input]) -> Vec<Input> {
    values
        .iter()
        .map(|value| Input {
            index: value.index,
            local: value.local,
            ty: value.ty.clone(),
            domain: domain::of(&value.ty),
        })
        .collect()
}
