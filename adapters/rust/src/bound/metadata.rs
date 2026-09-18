// Metadata loading rejects empty, malformed, or unsupported Kani output.
// It returns complete crate records from the current model build only.

pub use super::metadata_kind::{CrateMetadata, HarnessMetadata};
use super::{metadata_paths, Error};
use std::path::Path;

pub fn read_all(root: &Path) -> Result<Vec<CrateMetadata>, Error> {
    let paths = metadata_paths::all(root)?;
    if paths.is_empty() {
        return Err(Error::Model {
            reason: format!("Kani produced no metadata under {}", root.display()),
        });
    }
    paths.iter().map(|path| read(path)).collect()
}

fn read(path: &Path) -> Result<CrateMetadata, Error> {
    let file = std::fs::File::open(path).map_err(|error| Error::Artifact {
        path: path.to_path_buf(),
        reason: error.to_string(),
    })?;
    let mut metadata: CrateMetadata =
        serde_json::from_reader(file).map_err(|error| Error::Json {
            origin: path.display().to_string(),
            reason: error.to_string(),
        })?;
    metadata.source = path.to_path_buf();
    if metadata.unsupported_features.is_empty() {
        return Ok(metadata);
    }
    Err(Error::Model {
        reason: format!(
            "{} reports unsupported Kani features: {:?}",
            path.display(),
            metadata.unsupported_features
        ),
    })
}
