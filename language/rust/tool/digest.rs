// Purpose: names a file by the hash of its bytes.
// Never:   hashes a path or a name instead of the file's content.
use crate::blocked::Blocked;
use crate::record::FileDigest;
use sha2::{Digest, Sha256};
use std::path::Path;

/// Reads the file and returns its path with the SHA-256 of its bytes.
pub fn digest_of(path: &Path) -> Result<FileDigest, Blocked> {
    let bytes = std::fs::read(path).map_err(|error| Blocked::Unreadable {
        what: path.display().to_string(),
        cause: error.to_string(),
    })?;
    let hash = Sha256::digest(&bytes);
    Ok(FileDigest {
        path: path.display().to_string(),
        sha256: format!("{hash:x}"),
    })
}
