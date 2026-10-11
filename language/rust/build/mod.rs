// Purpose: groups Cargo requests, reports, and debug-information reading.
// Never: changes the project's requested compilation.
pub mod cargo_build;
pub(crate) mod cargo_build_result;
mod cargo_build_types;
pub mod cargo_compiler;
mod cargo_compiler_crate;
mod cargo_compiler_libraries;
mod cargo_dynamic_names;
mod cargo_finished;
pub mod cargo_handover;
pub mod cargo_library;
pub mod cargo_loaded_libraries;
pub mod cargo_messages;
pub mod cargo_observation;
mod cargo_refusal;
pub mod cargo_request;
pub mod cargo_settings;
mod cargo_target;
mod cargo_wrapper;
mod cargo_wrapper_files;
pub mod debug_info;
