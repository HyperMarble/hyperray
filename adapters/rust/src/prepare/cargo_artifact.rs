// Select exactly one generated archive from Cargo's structured build output.
// A zero process exit alone cannot establish that the binding exists.
use super::cargo_event::{self, Event};
use std::io::BufRead;
use std::path::{Path, PathBuf};

pub fn select(output: impl BufRead, manifest: &Path) -> Result<PathBuf, String> {
    let mut archives = Vec::new();
    let mut finishes = Vec::new();
    for line in output.lines() {
        let line = line.map_err(|error| error.to_string())?;
        match cargo_event::parse(&line, manifest)? {
            Event::Archive(path) => archives.push(path),
            Event::Finished(success) => finishes.push(success),
            Event::Diagnostic => (),
        }
    }
    if finishes != [true] {
        return Err("Cargo output must contain one successful build-finished event".into());
    }
    match archives.as_slice() {
        [archive] => Ok(archive.clone()),
        _ => Err("Cargo output must contain exactly one generated binding archive".into()),
    }
}

#[cfg(test)]
#[path = "cargo_artifact_test.rs"]
mod tests;

#[cfg(test)]
#[path = "cargo_target_test.rs"]
mod target_tests;
