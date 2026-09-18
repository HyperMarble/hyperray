// The compiler type structure needed for input domains. Printed type names
// never decide a domain.

use super::ScalarKind;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum MirType {
    Scalar {
        class: ScalarKind,
        bits: u16,
    },
    Array {
        length: Option<u64>,
        element: Box<MirType>,
    },
    Slice {
        element: Box<MirType>,
    },
    Str,
    Reference {
        mutable: bool,
        target: Box<MirType>,
    },
    Tuple {
        fields: Vec<MirType>,
    },
    Adt,
    RawPointer,
    Never,
    Param,
    Alias,
    Bound,
    Callable,
    Coroutine,
    Opaque,
}
