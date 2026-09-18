// A counted-loop fixture states the complete exit-related evidence chain.
use super::{assign, block, input, item, place, scalar};
use hyperray_rust::mir::Terminator;
use hyperray_rust::mir::{Block, Branch, Input, Item, MirType, Operation, Rvalue, ScalarKind};

pub fn counted_loop() -> Item {
    item(
        "crate::counted",
        Some("crate"),
        vec![loop_limit()],
        vec![
            initialize_block(),
            compare_block(),
            progress_block(),
            block(Vec::new(), Terminator::End),
        ],
    )
}

fn loop_limit() -> Input {
    input(
        0,
        1,
        MirType::Scalar {
            class: ScalarKind::Unsigned,
            bits: 64,
        },
    )
}

fn initialize_block() -> Block {
    let initialize = assign(
        2,
        Rvalue::Use {
            operand: scalar("0"),
        },
    );
    block(vec![initialize], Terminator::Goto { target: 1 })
}

fn compare_block() -> Block {
    let compare = assign(
        3,
        Rvalue::Binary {
            operation: Operation::Less,
            left: place(2),
            right: place(1),
            checked: false,
        },
    );
    block(
        vec![compare],
        Terminator::Switch {
            discriminant: place(3),
            branches: vec![Branch {
                value: "0".to_string(),
                target: 3,
            }],
            otherwise: 2,
        },
    )
}

fn progress_block() -> Block {
    let progress = assign(
        2,
        Rvalue::Binary {
            operation: Operation::Add,
            left: place(2),
            right: scalar("1"),
            checked: true,
        },
    );
    block(vec![progress], Terminator::Goto { target: 1 })
}
