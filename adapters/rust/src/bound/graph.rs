// Natural loop blocks grouped by compiler back-edge header. This keeps nested
// loop headers separate.

use super::edge;
use super::{Edge, Error};
use crate::mir::Body;
use std::collections::BTreeMap;

pub struct LoopGraph {
    pub header: usize,
    pub blocks: Vec<usize>,
    pub back_edges: Vec<Edge>,
}

pub fn loops(item: &str, body: &Body) -> Result<Vec<LoopGraph>, Error> {
    let found = edge::find(item, body)?;
    let mut by_header: BTreeMap<usize, Vec<Edge>> = BTreeMap::new();
    for edge in found.back_edges {
        by_header.entry(edge.to).or_default().push(edge);
    }
    let predecessors = predecessors(body);
    Ok(by_header
        .into_iter()
        .map(|(header, back_edges)| LoopGraph {
            header,
            blocks: natural_blocks(header, &back_edges, &predecessors, &found.reachable),
            back_edges,
        })
        .collect())
}

fn predecessors(body: &Body) -> Vec<Vec<usize>> {
    let mut result = vec![Vec::new(); body.blocks.len()];
    for (from, block) in body.blocks.iter().enumerate() {
        for to in block.terminator.successors() {
            result[to].push(from);
        }
    }
    result
}

fn natural_blocks(
    header: usize,
    edges: &[Edge],
    predecessors: &[Vec<usize>],
    reachable: &[bool],
) -> Vec<usize> {
    let mut included = vec![false; predecessors.len()];
    included[header] = true;
    let mut pending: Vec<usize> = edges.iter().map(|edge| edge.from).collect();
    while let Some(block) = pending.pop() {
        if included[block] || !reachable[block] {
            continue;
        }
        included[block] = true;
        pending.extend(predecessors[block].iter().copied());
    }
    included
        .iter()
        .enumerate()
        .filter_map(|(block, present)| present.then_some(block))
        .collect()
}
