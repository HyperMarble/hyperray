// Kani metadata names an intermediate symtab file in the measured tool output.
// Stage 3 accepts only the final linked GOTO file that CBMC can read safely.

use super::Error;
use std::path::{Path, PathBuf};

pub fn linked(path: &Path) -> Result<PathBuf, Error> {
    let text = path.to_str().ok_or_else(|| Error::Artifact {
        path: path.to_path_buf(),
        reason: "Kani GOTO path is not UTF-8".to_string(),
    })?;
    let candidate = match text.strip_suffix(".symtab.out") {
        Some(base) => PathBuf::from(format!("{base}.out")),
        None => path.to_path_buf(),
    };
    if candidate.is_file() {
        return Ok(candidate);
    }
    Err(Error::Artifact {
        path: candidate,
        reason: "final linked Kani GOTO model is missing".to_string(),
    })
}
