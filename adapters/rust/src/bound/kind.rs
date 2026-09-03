// The bound row for one manifest function (stage3.md Phase D step 2).
// One variant per pile; a function Charon gave no signature for carries
// stage 1's Charon status as its reason.

use super::limit::Limit;
use super::size::Size;
use serde::Serialize;

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum Bound {
    None,
    FixedWidth {
        inputs: Vec<Size>,
    },
    Sized {
        inputs: Vec<Size>,
        limits: Vec<Limit>,
    },
    Loop {
        inputs: Vec<Size>,
        limits: Vec<Limit>,
        reason: Option<String>,
    },
    Unbuildable {
        input: usize,
        ty_kind: String,
        inputs: Vec<Size>,
    },
    NotInCharon {
        reason: String,
    },
}

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
pub struct Row {
    pub path: String,
    pub name: String,
    pub start_line: u32,
    pub bound: Bound,
}
