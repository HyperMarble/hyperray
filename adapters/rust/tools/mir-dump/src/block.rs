// Compiler blocks converted to assignments and one normal terminator. Each
// source line conversion reports overflow as an error.

use crate::{place, rvalue, terminator};
use hyperray_rust::mir::{Block, Statement};
use rustc_public::mir::{Body, StatementKind};

pub fn of(body: &Body) -> Result<Vec<Block>, String> {
    body.blocks
        .iter()
        .map(|block| {
            Ok(Block {
                statements: statements(block)?,
                terminator: terminator::of(&block.terminator.kind),
                terminator_line: line(block.terminator.source_info.span.get_lines().start_line)?,
            })
        })
        .collect()
}

fn statements(block: &rustc_public::mir::BasicBlock) -> Result<Vec<Statement>, String> {
    block
        .statements
        .iter()
        .filter_map(|statement| {
            let StatementKind::Assign(target, value) = &statement.kind else {
                return None;
            };
            Some(
                line(statement.source_info.span.get_lines().start_line).map(|line| Statement {
                    target: place::of(target),
                    value: rvalue::of(value),
                    line,
                }),
            )
        })
        .collect()
}

fn line(value: usize) -> Result<u32, String> {
    u32::try_from(value).map_err(|_| format!("source line {value} does not fit in u32"))
}
