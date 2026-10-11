// Preserves the owner's request-level wrapper choices through the build flow.
// Never accepts a successful build that silently dropped a selected wrapper.
#[path = "request_config/compare.rs"]
mod compare;
#[path = "request_config/project.rs"]
mod project;
#[path = "request_config/record.rs"]
mod record;
mod sample_project;
#[path = "compiler_wrapper/project.rs"]
mod wrappers;

#[path = "request_config/disabled.rs"]
mod disabled;
#[path = "request_config/environment.rs"]
mod environment;
#[path = "request_config/file.rs"]
mod file;
#[path = "request_config/inline.rs"]
mod inline;
