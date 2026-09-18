// Assignment and call definitions for one MIR local. Ambiguous definitions
// return no single source.

use crate::mir::{Body, Statement};

#[derive(Clone, Copy)]
pub struct Assignment<'a> {
    pub block: usize,
    pub index: usize,
    pub statement: &'a Statement,
}

pub fn assignments(body: &Body, local: usize) -> Vec<Assignment<'_>> {
    body.blocks
        .iter()
        .enumerate()
        .flat_map(|(block, data)| {
            data.statements
                .iter()
                .enumerate()
                .filter(move |(_, statement)| {
                    statement.target.local == local && statement.target.projection.is_empty()
                })
                .map(move |(index, statement)| Assignment {
                    block,
                    index,
                    statement,
                })
        })
        .collect()
}

pub fn reaching(body: &Body, block: usize, before: usize, local: usize) -> Option<Assignment<'_>> {
    let local_value = body.blocks[block]
        .statements
        .get(..before)?
        .iter()
        .enumerate()
        .rev()
        .find(|(_, statement)| {
            statement.target.local == local && statement.target.projection.is_empty()
        })
        .map(|(index, statement)| Assignment {
            block,
            index,
            statement,
        });
    local_value.or_else(|| unique_assignment(body, local))
}

pub fn unique_assignment(body: &Body, local: usize) -> Option<Assignment<'_>> {
    let found = assignments(body, local);
    match found.as_slice() {
        [assignment] => Some(*assignment),
        _ => None,
    }
}
