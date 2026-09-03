// A loop in ULLBC is a cycle in the control-flow graph reachable from
// block 0. Block ids are not in control-flow order (measured: a
// one-line `From::from` has block 10 jumping to block 3 through its
// drop-cleanup chain), so "target ≤ own id" is not the test; a
// depth-first walk marking blocks on the current path is. `on_unwind`
// edges are followed too — a cycle only through unwind blocks does not
// exist in MIR, and following them costs nothing.

use super::block::{TerminatorKind, Unstructured};

#[derive(Clone, Copy, PartialEq)]
enum Mark {
    New,
    OnPath,
    Done,
}

pub fn has_back_edge(body: &Unstructured) -> bool {
    let mut state = vec![Mark::New; body.body.len()];
    !body.body.is_empty() && cycle_from(body, 0, &mut state)
}

// `block` is always a valid index: 0 is checked by the caller, and every
// later one came back `Some` from `state.get`.
fn cycle_from(body: &Unstructured, block: usize, state: &mut [Mark]) -> bool {
    state[block] = Mark::OnPath;
    for next in targets(&body.body[block].terminator.kind) {
        let next = next as usize;
        let mark = state.get(next).copied();
        if mark == Some(Mark::OnPath) {
            return true;
        }
        if mark == Some(Mark::New) && cycle_from(body, next, state) {
            return true;
        }
    }
    state[block] = Mark::Done;
    false
}

fn targets(kind: &TerminatorKind) -> Vec<u32> {
    match kind {
        TerminatorKind::Goto { target } => vec![*target],
        TerminatorKind::Switch { branches, .. } => branches.clone(),
        TerminatorKind::Call {
            target, on_unwind, ..
        }
        | TerminatorKind::Drop {
            target, on_unwind, ..
        }
        | TerminatorKind::Assert {
            target, on_unwind, ..
        } => vec![*target, *on_unwind],
        TerminatorKind::InlineAsm {
            targets, on_unwind, ..
        } => {
            let mut all = targets.clone();
            all.push(*on_unwind);
            all
        }
        TerminatorKind::Abort(_) | TerminatorKind::Return | TerminatorKind::UnwindResume => vec![],
    }
}
