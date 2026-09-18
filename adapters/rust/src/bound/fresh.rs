// Each model build uses a new artifact directory, so stale files cannot pass.
// Directory exhaustion and file-system failure remain visible to the caller.

use super::Error;
use std::io::ErrorKind;
use std::path::{Path, PathBuf};

pub fn directory(root: &Path) -> Result<PathBuf, Error> {
    std::fs::create_dir_all(root).map_err(|error| artifact(root, error.to_string()))?;
    let root = std::fs::canonicalize(root).map_err(|error| artifact(root, error.to_string()))?;
    let mut sequence = 0_u64;
    loop {
        let candidate = root.join(format!("model-{sequence}"));
        match std::fs::create_dir(&candidate) {
            Ok(()) => return Ok(candidate),
            Err(error) if error.kind() == ErrorKind::AlreadyExists => {
                sequence = sequence.checked_add(1).ok_or_else(|| Error::Model {
                    reason: "Stage 3 model directory sequence exhausted u64".to_string(),
                })?;
            }
            Err(error) => return Err(artifact(&candidate, error.to_string())),
        }
    }
}

fn artifact(path: &Path, reason: String) -> Error {
    Error::Artifact {
        path: path.to_path_buf(),
        reason,
    }
}
