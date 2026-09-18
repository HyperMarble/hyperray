// A new workspace-wrapper path for each compiler run. Cargo then rebuilds
// package targets while it keeps dependency artifacts in its cache.

use std::io::ErrorKind;
use std::path::{Path, PathBuf};

pub fn driver_link(driver: &Path, output_dir: &Path) -> Result<PathBuf, String> {
    let directory = output_dir.join("drivers");
    std::fs::create_dir_all(&directory)
        .map_err(|error| format!("cannot create {}: {error}", directory.display()))?;
    let mut sequence = 0_u64;
    loop {
        let link = directory.join(format!("mir-dump-{sequence}"));
        match std::os::unix::fs::symlink(driver, &link) {
            Ok(()) => return Ok(link),
            Err(error) if error.kind() == ErrorKind::AlreadyExists => {
                sequence = next(sequence)?;
            }
            Err(error) => return Err(format!("cannot create {}: {error}", link.display())),
        }
    }
}

fn next(sequence: u64) -> Result<u64, String> {
    sequence
        .checked_add(1)
        .ok_or_else(|| "compiler run sequence exhausted u64".to_string())
}
