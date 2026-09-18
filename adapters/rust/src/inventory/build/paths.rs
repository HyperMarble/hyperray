// Own the output names used by one fresh build reservation.

use std::path::{Path, PathBuf};

pub(super) struct Paths {
    pub(super) object: PathBuf,
    pub(super) dep_info: PathBuf,
    pub(super) inventory: PathBuf,
    pub(super) elf: PathBuf,
    pub(super) inventory_directory: PathBuf,
    pub(super) compiler_directory: PathBuf,
}

impl Paths {
    pub(super) fn new(directory: &Path) -> Self {
        let inventory_directory = directory.join("inventory");
        let compiler_directory = directory.join("compiler");
        Self {
            object: directory.join("program.o"),
            dep_info: directory.join("program.d"),
            inventory: inventory_directory.join("inventory.json"),
            elf: directory.join("program.elf"),
            inventory_directory,
            compiler_directory,
        }
    }
}
