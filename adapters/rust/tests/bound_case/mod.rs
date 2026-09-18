// Public MIR constructors keep Stage 3 tests focused on declared behavior.
// They must not hide an expected control-flow edge.

mod cycles;
mod interior;
mod linear;

pub use cycles::{impossible_resume, iterator_loop};
pub use interior::interior_comparison;
pub use linear::counted_loop;

use hyperray_rust::mir::{
    Block, Body, Input, Item, Kind, MirType, Operand, Place, Rvalue, Scalar, ScalarKind, Statement,
    Terminator,
};

pub fn item(name: &str, parent: Option<&str>, inputs: Vec<Input>, blocks: Vec<Block>) -> Item {
    Item {
        name: name.to_string(),
        parent: parent.map(str::to_string),
        kind: Kind::Function,
        file: "src/lib.rs".to_string(),
        start_line: 1,
        end_line: 8,
        value: None,
        inputs,
        body: Some(Body {
            local_count: 8,
            blocks,
        }),
    }
}

pub fn input(index: usize, local: usize, ty: MirType) -> Input {
    Input { index, local, ty }
}

pub fn scalar(value: &str) -> Operand {
    Operand::Scalar(Scalar {
        value: value.to_string(),
        class: ScalarKind::Unsigned,
        bits: 64,
    })
}

pub fn place(local: usize) -> Operand {
    Operand::Place(target(local))
}

pub fn target(local: usize) -> Place {
    Place {
        local,
        projection: Vec::new(),
    }
}

pub fn assign(local: usize, value: Rvalue) -> Statement {
    Statement {
        target: target(local),
        value,
        line: 3,
    }
}

pub fn block(statements: Vec<Statement>, terminator: Terminator) -> Block {
    Block {
        statements,
        terminator,
        terminator_line: 4,
    }
}
