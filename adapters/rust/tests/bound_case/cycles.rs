// Cycle fixtures distinguish impossible compiler edges from real iterator
// state transitions. Neither may receive invented linear evidence.

use super::{block, item, place, target};
use hyperray_rust::mir::{Branch, Item, Operand, Scalar, ScalarKind, Terminator};

pub fn impossible_resume() -> Item {
    item(
        "crate::resume",
        Some("crate"),
        Vec::new(),
        vec![block(
            Vec::new(),
            Terminator::Assert {
                condition: Operand::Scalar(Scalar {
                    value: "false".to_string(),
                    class: ScalarKind::Boolean,
                    bits: 1,
                }),
                expected: true,
                target: 0,
            },
        )],
    )
}

pub fn iterator_loop() -> Item {
    item(
        "crate::iterate",
        Some("crate"),
        Vec::new(),
        vec![
            block(
                Vec::new(),
                Terminator::Call {
                    function: Some("Iterator::next".to_string()),
                    arguments: Vec::new(),
                    destination: target(2),
                    target: Some(1),
                },
            ),
            block(
                Vec::new(),
                Terminator::Switch {
                    discriminant: place(2),
                    branches: vec![Branch {
                        value: "0".to_string(),
                        target: 2,
                    }],
                    otherwise: 0,
                },
            ),
            block(Vec::new(), Terminator::End),
        ],
    )
}
