// Purpose: records config files with Cargo's recursive include order.
// Never:   hides a malformed or required-missing include.
#[path = "config_cli_includes.rs"]
mod cli;
#[path = "config_include_load.rs"]
mod load;
#[path = "config_include_paths.rs"]
mod paths;

use crate::blocked::Blocked;
use crate::digest::digest_of;
use crate::record::FileDigest;
use std::collections::HashSet;
use std::path::{
    Path,
    PathBuf, //
};

#[derive(Default)]
pub(super) struct Collector {
    files: Vec<FileDigest>,
    known: HashSet<PathBuf>,
    active: HashSet<PathBuf>,
    missing_optional: Vec<String>,
}

impl Collector {
    pub(super) fn load(&mut self, file: &Path) -> Result<(), Blocked> {
        let identity = file
            .canonicalize()
            .map_err(|error| paths::unreadable(file, error))?;
        if !self.active.insert(identity.clone()) {
            return Err(paths::cycle(file));
        }
        self.load_includes(file)?;
        self.active.remove(&identity);
        if self.known.insert(identity) {
            self.files.push(digest_of(file)?);
        }
        Ok(())
    }

    pub(super) fn finish(self) -> super::ConfigReport {
        super::ConfigReport {
            files: self.files,
            missing_optional: self.missing_optional,
        }
    }
}
