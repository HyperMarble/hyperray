// Build artifacts in a new directory and preserve unsuccessful build evidence.
// Existing output directories and passing artifacts must never be overwritten.
use super::{compile, generate, input_bits, versions, Prepared, Request, RuntimeRequest};
use std::fs;

pub fn build(request: &Request) -> Result<Prepared, String> {
    request.validate()?;
    if !cfg!(target_os = "macos") {
        return Err("native preparation currently requires macOS".into());
    }
    fs::create_dir(&request.directory)
        .map_err(|error| format!("create output directory: {error}"))?;
    build_directory(request)
        .map_err(|error| format!("prepare in {}: {error}", request.directory.display()))
}

fn build_directory(request: &Request) -> Result<Prepared, String> {
    let tools = versions::collect(request)?;
    generate::write(request)?;
    compile::libraries(request)?;
    compile::checker(request)?;
    super::paths::executable_artifact(&request.directory.join("search"))?;
    super::paths::executable_artifact(&request.directory.join("replay"))?;
    let limits = &request.limits;
    let prepared = Prepared {
        execution: RuntimeRequest {
            executable: request.directory.join("search"),
            arguments: vec![
                format!("-m{}", limits.search_depth),
                format!("-w{}", limits.hash_bits),
            ],
            directory: request.directory.clone(),
            environment: Vec::new(),
            timeout_nanoseconds: limits.timeout_nanoseconds()?,
            output_limit_bytes: limits.output_limit_bytes,
            memory_budget_bytes: limits.memory_budget_bytes,
        },
        replay_executable: request.directory.join("replay"),
        input_bits: input_bits(limits.minimum, limits.maximum)?,
        minimum: limits.minimum,
        maximum: limits.maximum,
        tools,
    };
    let encoded = serde_json::to_vec_pretty(&prepared).map_err(|error| error.to_string())?;
    fs::write(request.directory.join("prepared.json"), encoded)
        .map_err(|error| error.to_string())?;
    Ok(prepared)
}
