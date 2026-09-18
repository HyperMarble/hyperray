// Interpret Cargo metadata, not Rust program behavior.
// Unrelated package artifacts cannot replace the generated binding.
use serde::Deserialize;
use serde_json::Value;
use std::path::{Path, PathBuf};

pub enum Event {
    Archive(PathBuf),
    Finished(bool),
    Diagnostic,
}

#[derive(Deserialize)]
struct Artifact {
    manifest_path: PathBuf,
    target: Target,
    filenames: Vec<PathBuf>,
}

#[derive(Deserialize)]
struct Target {
    name: String,
    crate_types: Vec<String>,
}

pub fn parse(line: &str, manifest: &Path) -> Result<Event, String> {
    let value: Value =
        serde_json::from_str(line).map_err(|error| format!("Cargo JSON: {error}"))?;
    match value["reason"].as_str() {
        Some("compiler-artifact") => archive(value, manifest),
        Some("build-finished") => value["success"]
            .as_bool()
            .map(Event::Finished)
            .ok_or("Cargo build-finished event lacks success".into()),
        Some("compiler-message" | "build-script-executed") => Ok(Event::Diagnostic),
        Some(reason) => Err(format!("unsupported Cargo event: {reason}")),
        None => Err("Cargo event lacks a reason".into()),
    }
}

fn archive(value: Value, manifest: &Path) -> Result<Event, String> {
    let artifact: Artifact = serde_json::from_value(value).map_err(|error| error.to_string())?;
    if artifact.manifest_path != manifest {
        return Ok(Event::Diagnostic);
    }
    if artifact.target.name != "binding" || artifact.target.crate_types != ["staticlib"] {
        return Err("generated Cargo artifact has the wrong target".into());
    }
    match artifact.filenames.as_slice() {
        [archive] if archive.is_absolute() && archive.extension() == Some("a".as_ref()) => {
            Ok(Event::Archive(archive.clone()))
        }
        _ => Err("generated Cargo artifact must name one absolute static archive".into()),
    }
}
