// Construct public inputs from repository fixtures and the current test executable.
// This helper does not start the placeholder tool executable.
use hyperray_rust::prepare::{FunctionSource, Limits, Request, Tools};
use std::path::PathBuf;

pub fn request() -> Result<Request, Box<dyn std::error::Error>> {
    let root = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../fixtures/rust/preparation");
    let tool = std::env::current_exe()?;
    Ok(Request {
        cargo: None,
        subject: FunctionSource {
            source: root.join("subject.rs"),
            function: "workflow::solve".into(),
        },
        requirement: FunctionSource {
            source: root.join("requirements.rs"),
            function: "transformed".into(),
        },
        tools: Tools {
            rustc: tool.clone(),
            spin: tool.clone(),
            clang: tool,
        },
        directory: std::env::temp_dir(),
        optimization: 0,
        limits: Limits {
            minimum: 13,
            maximum: 23,
            search_depth: 100,
            hash_bits: 18,
            state_memory_mib: 32,
            timeout_ms: 10000,
            output_limit_bytes: 1000000,
            memory_budget_bytes: 100000000,
        },
    })
}
