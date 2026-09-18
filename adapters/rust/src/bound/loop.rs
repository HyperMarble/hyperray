// One structural cycle report from the extracted compiler graph.
// It carries no inferred numeric bound or proof verdict.

use super::Expression;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct Loop {
    pub item: String,
    pub line: u32,
    pub header: usize,
    pub blocks: Vec<usize>,
    pub back_edges: Vec<Edge>,
    pub exits: Vec<Exit>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct Edge {
    pub from: usize,
    pub to: usize,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct Exit {
    pub block: usize,
    pub line: u32,
    pub condition: Expression,
    pub repeat_targets: Vec<usize>,
    pub exit_targets: Vec<usize>,
}
