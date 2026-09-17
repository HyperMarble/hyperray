// The architecture definition, read from disk. One read serves every stage
// of a proof. It must not be re-read per instruction or per stage.
use std::path::Path;

/// A model file that has been read, ready to be initialised once.
#[derive(Debug)]
pub struct Architecture {
    pub bytes: Vec<u8>,
    pub source: String,
}

/// The architecture definition at `path`.
///
/// An empty file is a failure, not an empty model.
pub fn read(path: &Path) -> Result<Architecture, String> {
    let bytes = std::fs::read(path).map_err(|error| format!("{}: {error}", path.display()))?;
    if bytes.is_empty() {
        return Err(format!("{}: architecture file is empty", path.display()));
    }
    Ok(Architecture {
        bytes,
        source: path.display().to_string(),
    })
}
