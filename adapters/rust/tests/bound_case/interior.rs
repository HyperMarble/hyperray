// An interior comparison fixture contains a tempting constant that does not
// control the loop exit. That constant must not become a proposal.

use super::{assign, block, input, item, place, scalar};
use hyperray_rust::mir::{
    Branch, Item, MirType, Operand, Operation, Rvalue, ScalarKind, Terminator,
};

pub fn interior_comparison() -> Item {
    let value = input(
        0,
        1,
        MirType::Scalar {
            class: ScalarKind::Unsigned,
            bits: 64,
        },
    );
    let compare = assign(
        3,
        Rvalue::Binary {
            operation: Operation::Greater,
            left: place(1),
            right: scalar("3"),
            checked: false,
        },
    );
    let progress = assign(
        2,
        Rvalue::Binary {
            operation: Operation::Add,
            left: place(2),
            right: scalar("1"),
            checked: true,
        },
    );
    item(
        "crate::interior",
        Some("crate"),
        vec![value],
        vec![
            block(Vec::new(), Terminator::Goto { target: 1 }),
            branch(place(3), 2, 3, vec![compare]),
            block(Vec::new(), Terminator::Goto { target: 4 }),
            block(Vec::new(), Terminator::Goto { target: 4 }),
            branch(Operand::Other, 6, 5, Vec::new()),
            block(vec![progress], Terminator::Goto { target: 1 }),
            block(Vec::new(), Terminator::End),
        ],
    )
}

fn branch(
    discriminant: Operand,
    zero: usize,
    otherwise: usize,
    statements: Vec<hyperray_rust::mir::Statement>,
) -> hyperray_rust::mir::Block {
    block(
        statements,
        Terminator::Switch {
            discriminant,
            branches: vec![Branch {
                value: "0".to_string(),
                target: zero,
            }],
            otherwise,
        },
    )
}
