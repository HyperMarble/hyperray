// Reads includes of an inline Cargo configuration from the command directory.
// Never treats the command line as a file that does not exist.
use super::{
    paths,
    Collector, //
};
use crate::blocked::Blocked;
use std::path::Path;

impl Collector {
    pub(in super::super) fn load_inline(
        &mut self,
        root: &Path,
        table: &toml::Table,
    ) -> Result<(), Blocked> {
        let Some(value) = table.get("include") else {
            return Ok(());
        };
        for (file, optional) in paths::include_paths(value, root)? {
            let file = root.join(file);
            let exists = file
                .try_exists()
                .map_err(|error| paths::unreadable(&file, error))?;
            if !exists && optional {
                self.missing_optional.push(file.display().to_string());
                continue;
            }
            self.load(&file)?;
        }
        Ok(())
    }
}
