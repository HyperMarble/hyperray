// Structural cycle diagnostics come from compiler control-flow edges.
// They must not infer numeric limits from source patterns.

use super::{exit, graph, Error, Loop};
use crate::mir::{Body, Item};

pub fn analyze(item: &Item, body: &Body) -> Result<Vec<Loop>, Error> {
    let graphs = graph::loops(&item.name, body)?;
    Ok(graphs
        .into_iter()
        .map(|graph| loop_from(item, body, graph))
        .collect())
}

fn loop_from(item: &Item, body: &Body, graph: graph::LoopGraph) -> Loop {
    let exits = exit::conditions(body, &item.inputs, &graph);
    Loop {
        item: item.name.clone(),
        line: body.blocks[graph.header].terminator_line,
        header: graph.header,
        blocks: graph.blocks,
        back_edges: graph.back_edges,
        exits,
    }
}
