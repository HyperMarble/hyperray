// A MIR place identifies one local and every projection from it. Field zero
// is significant for the value part of checked arithmetic.

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq, Hash)]
#[serde(deny_unknown_fields)]
pub struct Place {
    pub local: usize,
    pub projection: Vec<Projection>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq, Hash)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum Projection {
    Dereference,
    Field {
        index: usize,
    },
    Index {
        local: usize,
    },
    ConstantIndex {
        offset: u64,
        minimum_length: u64,
        from_end: bool,
    },
    Subslice {
        from: u64,
        to: u64,
        from_end: bool,
    },
    Downcast,
    OpaqueCast,
}
