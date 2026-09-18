// Prepared artifacts expose the existing observer's public JSON request.
// A built checker is not a correctness result.
use serde::Serialize;
use std::path::PathBuf;

#[derive(Debug, Serialize)]
pub struct ToolVersion {
    pub tool: String,
    pub executable: PathBuf,
    pub version: String,
}

#[derive(Debug, Serialize)]
pub struct RuntimeRequest {
    pub executable: PathBuf,
    pub arguments: Vec<String>,
    pub directory: PathBuf,
    pub environment: Vec<String>,
    pub timeout_nanoseconds: i64,
    pub output_limit_bytes: u64,
    pub memory_budget_bytes: u64,
}

#[derive(Debug, Serialize)]
pub struct Prepared {
    pub execution: RuntimeRequest,
    pub replay_executable: PathBuf,
    pub input_bits: u32,
    pub minimum: u64,
    pub maximum: u64,
    pub tools: Vec<ToolVersion>,
}
