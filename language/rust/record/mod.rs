// Purpose: groups build records, build facts, and explicit refusal reasons.
// Never: changes the serialized build record.
pub mod blocked;
pub mod build_facts;
mod schema;

pub use schema::{
    Artifact,
    BuildRecord,
    FileDigest,
    Outcome,
    Settings,
    Toolchain, //
};
