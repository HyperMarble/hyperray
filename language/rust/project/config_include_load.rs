// Purpose: loads each required or present optional Cargo include.
// Never: hides a required include or records a missing optional file as loaded.
use super::{
    paths,
    Collector, //
};
use crate::blocked::Blocked;
use std::path::Path;

impl Collector {
    pub(super) fn load_includes(&mut self, file: &Path) -> Result<(), Blocked> {
        let configuration = paths::read_config(file)?;
        let Some(value) = configuration.get("include") else {
            return Ok(());
        };
        for (path, optional) in paths::include_paths(value, file)? {
            self.load_include(file, &path, optional)?;
        }
        Ok(())
    }

    fn load_include(&mut self, file: &Path, path: &Path, optional: bool) -> Result<(), Blocked> {
        let included = paths::resolve_include(file, path);
        let exists = included
            .try_exists()
            .map_err(|error| paths::unreadable(&included, error))?;
        if exists {
            return self.load(&included);
        }
        if optional {
            self.missing_optional.push(included.display().to_string());
            return Ok(());
        }
        self.load(&included)
    }
}
