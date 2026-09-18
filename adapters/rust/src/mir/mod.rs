// The versioned MIR boundary between the nightly compiler driver and the
// stable adapter. It must reject an incompatible schema.

mod block;
mod error;
mod flow;
mod instance;
mod inventory;
mod item;
mod operation;
mod place;
mod root_facts;
mod scalar;
mod terminator;
mod ty;
mod value;

pub use block::{Block, Statement};
pub use error::ReadError;
pub use instance::{CompilerInstance, CompilerInstanceKind};
pub use inventory::{RawBody, RawInstance, RawInventory, RawPosition};
pub use item::{Body, Dump, Input, Item, Kind};
pub use operation::Operation;
pub use place::{Place, Projection};
pub use root_facts::{ArgumentFact, RootFacts, ScalarFact, WrappingRange};
pub use scalar::{Scalar, ScalarKind};
pub use terminator::{Branch, Terminator};
pub use ty::MirType;
pub use value::{Operand, Rvalue};

pub const SCHEMA_VERSION: u32 = 4;

#[derive(serde::Deserialize)]
struct SchemaHeader {
    schema_version: u32,
}

pub fn read(file: std::fs::File) -> Result<Dump, ReadError> {
    use std::io::Seek;

    let mut reader = std::io::BufReader::new(file);
    let header: SchemaHeader = serde_json::from_reader(&mut reader).map_err(ReadError::Json)?;
    if header.schema_version != SCHEMA_VERSION {
        return Err(ReadError::Schema {
            expected: SCHEMA_VERSION,
            found: header.schema_version,
        });
    }
    reader
        .rewind()
        .map_err(|error| ReadError::Json(serde_json::Error::io(error)))?;
    serde_json::from_reader(reader).map_err(ReadError::Json)
}
