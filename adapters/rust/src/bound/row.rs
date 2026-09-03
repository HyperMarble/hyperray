// Phase D: one bound row per manifest function, manifest order, joined
// to Phase A by path + start_line (the stage-1 key). Charon emits more
// than one fun_decl at a closure's or a derive's line; the first whose
// span contains the function's start line is the function, the same
// choice stage 1's join makes.

use super::kind::{Bound, Row};
use super::pile::Pile;
use super::sort::Sorted;
use crate::extract::{Joined, Status};

pub fn rows(joined: &[Joined], sorted: &[Sorted]) -> Vec<Row> {
    joined
        .iter()
        .map(|j| Row {
            path: j.path.clone(),
            name: j.name.clone(),
            start_line: j.start_line,
            bound: bound_for(j, sorted),
        })
        .collect()
}

fn bound_for(j: &Joined, sorted: &[Sorted]) -> Bound {
    let hit = sorted
        .iter()
        .find(|s| s.path == j.path && s.start_line <= j.start_line && j.start_line <= s.end_line);
    match hit {
        Some(s) => bound_of(s),
        None => Bound::NotInCharon {
            reason: reason_of(&j.status),
        },
    }
}

fn bound_of(s: &Sorted) -> Bound {
    let inputs = s.inputs.clone();
    let limits = s.limits.clone();
    match &s.pile {
        Pile::NoBound => Bound::None,
        Pile::FixedWidth => Bound::FixedWidth { inputs },
        Pile::Sized => Bound::Sized { inputs, limits },
        Pile::Loop => Bound::Loop {
            inputs,
            reason: limits
                .is_empty()
                .then(|| "no evaluated constant is compared in this body".to_string()),
            limits,
        },
        Pile::Unbuildable { input, kind } => Bound::Unbuildable {
            input: *input,
            ty_kind: kind.clone(),
            inputs,
        },
    }
}

// Stage 1's words, verbatim. `Extracted` with no Phase-A row cannot
// happen (Phase A reads every fun_decl); it is still named, not hidden.
fn reason_of(status: &Status) -> String {
    match status {
        Status::Refused(reason) => reason.clone(),
        Status::Missing => "Charon has no declaration for this function".to_string(),
        Status::FileNotSeen => "Charon never saw this file".to_string(),
        Status::Extracted => "Charon extracted the body but no signature row matched".to_string(),
    }
}
