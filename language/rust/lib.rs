// Purpose: the Rust adapter: build a project its own way, record what came out.
// Never:   read instructions or prove anything; the loader and engine do that.
mod build;
pub mod project;
pub mod record;
mod tool;

pub use build::{
    cargo_build,
    cargo_messages,
    cargo_observation,
    cargo_request,
    debug_info, //
};
pub use project::config_files;
pub use record::{
    blocked,
    build_facts, //
};
pub use tool::{
    c_toolchain,
    digest,
    environment,
    native_argument,
    os,
    process_evidence,
    run,
    toolchain, //
};

use cargo_request::Arguments;
use record::Outcome;
use std::path::Path;

/// Builds the project with Cargo's default options and returns its record.
pub fn build_record(root: &Path) -> Outcome {
    build_record_with(root, &Arguments::default())
}

/// Passes the caller's native arguments to Cargo and returns the build record.
pub fn build_record_with(root: &Path, supplied: &Arguments) -> Outcome {
    match record::record_builder::build(root, supplied) {
        Ok(record) => Outcome::Built(Box::new(record)),
        Err(blocked) => Outcome::Blocked {
            reason: blocked.to_string(),
        },
    }
}
