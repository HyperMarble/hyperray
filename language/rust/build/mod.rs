// Purpose: groups Cargo requests, reports, and debug-information reading.
// Never: changes the project's requested compilation.
pub mod cargo_build;
pub(crate) mod cargo_build_result;
mod cargo_build_types;
mod cargo_members;
pub mod cargo_messages;
pub mod cargo_observation;
pub mod cargo_request;
pub mod debug_info;
