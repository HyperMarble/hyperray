// Purpose: the Rust adapter: build a project its own way, record what came out.
// Never:   read instructions or prove anything; the loader and engine do that.
pub mod blocked;
pub mod cargo_build;
pub mod digest;
pub mod os;
pub mod project;
pub mod record;
pub mod run;
pub mod toolchain;
