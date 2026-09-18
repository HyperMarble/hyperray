// Identity helpers hash files after the owner observes the process result.
// The caller never supplies or selects a digest.

use super::BuildError;
use sha2::{Digest, Sha256};
use std::path::Path;

pub fn file(path: &Path) -> Result<(String, u64), BuildError> {
    let bytes = std::fs::read(path)
        .map_err(|error| BuildError::HashFailed(path.to_path_buf(), error.to_string()))?;
    let size = bytes.len() as u64;
    Ok((digest(&bytes), size))
}

pub fn bytes(bytes: &[u8]) -> Result<String, BuildError> {
    Ok(digest(bytes))
}

fn digest(bytes: &[u8]) -> String {
    let mut hasher = Sha256::new();
    hasher.update(bytes);
    format!("{:x}", hasher.finalize())
}
