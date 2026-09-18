// Inventory compiler output paths without hiding filesystem errors.
// A failed directory read is not an empty successful inventory.
use std::path::{Path, PathBuf};

pub fn in_directory(directory: &Path) -> Result<Vec<PathBuf>, String> {
    let entries = std::fs::read_dir(directory)
        .map_err(|error| format!("read compiler output {}: {error}", directory.display()))?;
    let mut paths = Vec::new();
    for entry in entries {
        let path = entry.map_err(|error| error.to_string())?.path();
        if path.to_string_lossy().ends_with(".mir.json") {
            paths.push(path);
        }
    }
    paths.sort();
    Ok(paths)
}

#[cfg(test)]
mod tests {
    #[test]
    fn file_is_not_a_dump_directory() -> Result<(), std::io::Error> {
        let file = std::env::current_exe()?;
        assert!(super::in_directory(&file).is_err());
        Ok(())
    }
}
