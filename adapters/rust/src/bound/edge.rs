// Reachable back edges in one MIR body. A target outside the body is a
// structural error, not an omitted edge.

use super::{Edge, Error};
use crate::mir::Body;

#[derive(Clone, Copy, PartialEq)]
enum Mark {
    New,
    Active,
    Done,
}

pub struct Found {
    pub back_edges: Vec<Edge>,
    pub reachable: Vec<bool>,
}

pub fn find(item: &str, body: &Body) -> Result<Found, Error> {
    validate(item, body)?;
    let mut marks = vec![Mark::New; body.blocks.len()];
    let mut back_edges = Vec::new();
    if !body.blocks.is_empty() {
        visit(body, 0, &mut marks, &mut back_edges);
    }
    let reachable = marks.iter().map(|mark| *mark != Mark::New).collect();
    Ok(Found {
        back_edges,
        reachable,
    })
}

fn validate(item: &str, body: &Body) -> Result<(), Error> {
    let Some(edge) = invalid_edge(body) else {
        return Ok(());
    };
    Err(Error::Edge {
        item: item.to_string(),
        from: edge.from,
        to: edge.to,
        block_count: body.blocks.len(),
    })
}

fn invalid_edge(body: &Body) -> Option<Edge> {
    for (from, block) in body.blocks.iter().enumerate() {
        let Some(to) = block
            .terminator
            .successors()
            .into_iter()
            .find(|to| *to >= body.blocks.len())
        else {
            continue;
        };
        return Some(Edge { from, to });
    }
    None
}

fn visit(body: &Body, block: usize, marks: &mut [Mark], found: &mut Vec<Edge>) {
    marks[block] = Mark::Active;
    for next in body.blocks[block].terminator.successors() {
        match marks[next] {
            Mark::Active => found.push(Edge {
                from: block,
                to: next,
            }),
            Mark::New => visit(body, next, marks, found),
            Mark::Done => {}
        }
    }
    marks[block] = Mark::Done;
}
