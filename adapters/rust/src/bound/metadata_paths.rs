// Metadata discovery scans only the fresh Kani target directory for JSON files.
// It follows real directories and returns any read failure to the caller.

use super::Error;
use std::path::{Path, PathBuf};

pub fn all(root: &Path) -> Result<Vec<PathBuf>, Error> {
    let mut paths = Vec::new();
    visit(root, &mut paths)?;
    paths.sort();
    Ok(paths)
}

fn visit(directory: &Path, paths: &mut Vec<PathBuf>) -> Result<(), Error> {
    let entries = std::fs::read_dir(directory).map_err(|error| failure(directory, error))?;
    for result in entries {
        let entry = result.map_err(|error| failure(directory, error))?;
        let file_type = entry
            .file_type()
            .map_err(|error| failure(&entry.path(), error))?;
        if file_type.is_dir() {
            visit(&entry.path(), paths)?;
        }
        if file_type.is_file() && is_metadata(&entry.path()) {
            paths.push(entry.path());
        }
    }
    Ok(())
}

fn is_metadata(path: &Path) -> bool {
    path.file_name()
        .is_some_and(|name| name.to_string_lossy().ends_with(".kani-metadata.json"))
}

fn failure(path: &Path, error: std::io::Error) -> Error {
    Error::Artifact {
        path: path.to_path_buf(),
        reason: error.to_string(),
    }
}
