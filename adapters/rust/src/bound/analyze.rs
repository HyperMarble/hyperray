// Diagnostics for each compiler item retain structural cycles.
// Cycle presence does not establish a numeric bound or a proof verdict.

use super::{input, loop_analysis};
use super::{Error, Input, Loop};
use crate::mir::{Dump, Item};

#[derive(Debug, Clone)]
pub struct ItemAnalysis {
    pub crate_name: String,
    pub name: String,
    pub parent: Option<String>,
    pub path: String,
    pub start_line: u32,
    pub end_line: u32,
    pub inputs: Vec<Input>,
    pub loops: Vec<Loop>,
}

pub fn dump(value: &Dump) -> Result<Vec<ItemAnalysis>, Error> {
    value
        .items
        .iter()
        .map(|item| analyze(&value.crate_name, item))
        .collect()
}

pub fn analyze(crate_name: &str, item: &Item) -> Result<ItemAnalysis, Error> {
    let loops = match &item.body {
        Some(body) => loop_analysis::analyze(item, body)?,
        None => Vec::new(),
    };
    Ok(ItemAnalysis {
        crate_name: crate_name.to_string(),
        name: item.name.clone(),
        parent: item.parent.clone(),
        path: item.file.clone(),
        start_line: item.start_line,
        end_line: item.end_line,
        inputs: input::analyze(&item.inputs),
        loops,
    })
}
