// A loop in ULLBC is a cycle in the control-flow graph reachable from
// block 0. Block ids are not in control-flow order (measured: a
// one-line `From::from` has block 10 jumping to block 3 through its
// drop-cleanup chain), so "target ≤ own id" is not the test; a
// depth-first walk marking blocks on the current path is. `on_unwind`
// edges are followed too — a cycle only through unwind blocks does not
// exist in MIR, and following them costs nothing.

use super::block::{TerminatorKind, Unstructured};

pub fn has_back_edge(body: &Unstructured) -> bool {
    let n = body.body.len();
    let mut state = vec![Mark::New; n];
    let mut stack: Vec<(usize, Vec<u32>)> = vec![(0, vec![])];
    if n == 0 {
        return false;
    }
    state[0] = Mark::OnPath;
    stack[0].1 = targets(&body.body[0].terminator.kind);
    while let Some((block, pending)) = stack.last_mut() {
        let Some(next) = pending.pop() else {
            state[*block] = Mark::Done;
            stack.pop();
            continue;
        };
        let next = next as usize;
        match state.get(next) {
            Some(Mark::OnPath) => return true,
            Some(Mark::New) => {
                state[next] = Mark::OnPath;
                stack.push((next, targets(&body.body[next].terminator.kind)));
            }
            Some(Mark::Done) | None => {}
        }
    }
    false
}

#[derive(Clone, Copy, PartialEq)]
enum Mark {
    New,
    OnPath,
    Done,
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
