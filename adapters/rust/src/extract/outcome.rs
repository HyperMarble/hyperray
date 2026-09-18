// Compiler results retain feature selection and only newly produced dumps.
// Cached artifacts and directory errors cannot become current inventory evidence.
use super::{dumps, CompileRequest};
use crate::FeatureSelection;
use serde::Serialize;
use std::path::PathBuf;
use std::process::Output;

#[derive(Debug, Serialize, PartialEq, Eq)]
pub struct Run {
    pub driver: String,
    pub features: FeatureSelection,
    pub exit_code: Option<i32>,
    pub output_dir: String,
    pub dumps: usize,
    pub dump_paths: Vec<PathBuf>,
    pub log: String,
}

pub fn failed(request: &CompileRequest, log: String) -> Run {
    Run {
        driver: request.driver.display().to_string(),
        features: request.features.clone(),
        exit_code: None,
        output_dir: request.output_directory.display().to_string(),
        dumps: 0,
        dump_paths: Vec::new(),
        log,
    }
}

pub fn report(
    request: &CompileRequest,
    previous: &[PathBuf],
    result: std::io::Result<Output>,
) -> Result<Run, String> {
    let done = result.map_err(|error| format!("start Cargo: {error}"))?;
    let log = format!(
        "{}{}",
        String::from_utf8_lossy(&done.stdout),
        String::from_utf8_lossy(&done.stderr)
    );
    let dump_paths: Vec<PathBuf> = dumps::in_directory(&request.output_directory)?
        .into_iter()
        .filter(|path| !previous.contains(path))
        .collect();
    if done.status.success() && dump_paths.is_empty() {
        return Err(format!(
            "Cargo completed without fresh compiler inventory\n{log}"
        ));
    }
    Ok(Run {
        driver: request.driver.display().to_string(),
        features: request.features.clone(),
        exit_code: done.status.code(),
        output_dir: request.output_directory.display().to_string(),
        dumps: dump_paths.len(),
        dump_paths,
        log,
    })
}
