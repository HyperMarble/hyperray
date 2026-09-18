// Exit conditions whose compiler switch has one path inside a loop and one
// path outside it. Comparisons elsewhere are not limits.

use super::graph::LoopGraph;
use super::trace;
use super::Exit;
use crate::mir::{Body, Input, Terminator};

pub fn conditions(body: &Body, inputs: &[Input], graph: &LoopGraph) -> Vec<Exit> {
    graph
        .blocks
        .iter()
        .filter_map(|&block| condition(body, inputs, graph, block))
        .collect()
}

fn condition(body: &Body, inputs: &[Input], graph: &LoopGraph, block: usize) -> Option<Exit> {
    let Terminator::Switch { discriminant, .. } = &body.blocks[block].terminator else {
        return None;
    };
    let (repeat_targets, exit_targets) = targets(body, graph, block);
    if repeat_targets.is_empty() || exit_targets.is_empty() {
        return None;
    }
    Some(Exit {
        block,
        line: body.blocks[block].terminator_line,
        condition: trace::operand(body, inputs, block, discriminant),
        repeat_targets,
        exit_targets,
    })
}

fn targets(body: &Body, graph: &LoopGraph, block: usize) -> (Vec<usize>, Vec<usize>) {
    body.blocks[block]
        .terminator
        .successors()
        .into_iter()
        .partition(|target| graph.blocks.contains(target))
}
