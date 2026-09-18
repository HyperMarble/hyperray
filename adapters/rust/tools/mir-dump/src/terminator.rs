// Compiler terminators converted without unwind edges. Constant assertions
// retain their condition so the adapter can remove impossible success edges.

use crate::{operand, place};
use hyperray_rust::mir::{Branch, Terminator};
use rustc_public::mir::{SwitchTargets, TerminatorKind, TerminatorKind::*};

pub fn of(value: &TerminatorKind) -> Terminator {
    match value {
        Goto { target } => Terminator::Goto { target: *target },
        SwitchInt { discr, targets } => Terminator::Switch {
            discriminant: operand::of(discr),
            branches: switch_branches(targets),
            otherwise: targets.otherwise(),
        },
        Call {
            func,
            args,
            destination,
            target,
            ..
        } => Terminator::Call {
            function: operand::function_name(func),
            arguments: args.iter().map(operand::of).collect(),
            destination: place::of(destination),
            target: *target,
        },
        Assert {
            cond,
            expected,
            target,
            ..
        } => Terminator::Assert {
            condition: operand::of(cond),
            expected: *expected,
            target: *target,
        },
        Drop { target, .. } => Terminator::Drop { target: *target },
        InlineAsm { destination, .. } => Terminator::InlineAsm {
            target: *destination,
        },
        Return | Resume | Abort | Unreachable => Terminator::End,
    }
}

fn switch_branches(targets: &SwitchTargets) -> Vec<Branch> {
    targets
        .branches()
        .map(|(value, target)| Branch {
            value: value.to_string(),
            target,
        })
        .collect()
}
