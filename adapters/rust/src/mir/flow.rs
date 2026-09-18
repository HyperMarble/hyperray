// Successors are the reachable normal edges for one MIR terminator.
// A compiler-known failed assertion must not create a false cycle.

use super::{Branch, Operand, Terminator};

impl Terminator {
    pub fn successors(&self) -> Vec<usize> {
        match self {
            Self::Goto { target } | Self::Drop { target } => vec![*target],
            Self::Switch {
                discriminant,
                branches,
                otherwise,
            } => switch_successors(discriminant, branches, *otherwise),
            Self::Call { target, .. } | Self::InlineAsm { target } => {
                target.iter().copied().collect()
            }
            Self::Assert {
                condition,
                expected,
                target,
            } => assertion_successors(condition, *expected, *target),
            Self::End => Vec::new(),
        }
    }
}

fn switch_successors(value: &Operand, branches: &[Branch], otherwise: usize) -> Vec<usize> {
    let all = || {
        branches
            .iter()
            .map(|branch| branch.target)
            .chain([otherwise])
            .collect()
    };
    let Operand::Scalar(value) = value else {
        return all();
    };
    let Some(value) = value.switch_value() else {
        return all();
    };
    let target = branches
        .iter()
        .find(|branch| branch.value.parse::<u128>() == Ok(value))
        .map_or(otherwise, |branch| branch.target);
    vec![target]
}

fn assertion_successors(condition: &Operand, expected: bool, target: usize) -> Vec<usize> {
    match condition {
        Operand::Scalar(value) if value.boolean() != Some(expected) => Vec::new(),
        Operand::Place(_) | Operand::Function(_) | Operand::Other | Operand::Scalar(_) => {
            vec![target]
        }
    }
}
