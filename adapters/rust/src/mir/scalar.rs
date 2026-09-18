// A compiler scalar keeps its value, class, and bit width. The adapter must
// not reinterpret an unsigned bit pattern as a signed value.

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ScalarKind {
    Boolean,
    Character,
    Signed,
    Unsigned,
    Float,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct Scalar {
    pub value: String,
    pub class: ScalarKind,
    pub bits: u16,
}

impl Scalar {
    pub fn switch_value(&self) -> Option<u128> {
        match self.class {
            ScalarKind::Boolean if self.value == "false" => Some(0),
            ScalarKind::Boolean if self.value == "true" => Some(1),
            ScalarKind::Signed => self.value.parse::<i128>().ok().map(|value| value as u128),
            ScalarKind::Unsigned | ScalarKind::Character => self.value.parse().ok(),
            ScalarKind::Boolean | ScalarKind::Float => None,
        }
    }

    pub fn boolean(&self) -> Option<bool> {
        match (self.class, self.value.as_str()) {
            (ScalarKind::Boolean, "false") => Some(false),
            (ScalarKind::Boolean, "true") => Some(true),
            _ => None,
        }
    }
}
