// Purpose: makes a tiny real Cargo project on disk for one test.
// Never:   shares a project folder between tests; each name gets its own.
use std::path::PathBuf;
use std::process::Command;

/// Where the source goes and what it is.
pub struct Sample<'a> {
    pub name: &'a str,
    pub file: &'a str,
    pub source: &'a str,
    pub locked: bool,
}

/// Writes the project and, when `locked`, pins it with a Cargo.lock.
pub fn create(sample: &Sample<'_>) -> Result<PathBuf, String> {
    let root = PathBuf::from(env!("CARGO_TARGET_TMPDIR")).join(sample.name);
    if root.exists() {
        std::fs::remove_dir_all(&root).map_err(|error| error.to_string())?;
    }
    std::fs::create_dir_all(root.join("src")).map_err(|error| error.to_string())?;
    let manifest = format!(
        "[package]\nname = \"{}\"\nversion = \"0.1.0\"\nedition = \"2021\"\n",
        sample.name
    );
    std::fs::write(root.join("Cargo.toml"), manifest).map_err(|error| error.to_string())?;
    std::fs::write(root.join("src").join(sample.file), sample.source)
        .map_err(|error| error.to_string())?;
    if sample.locked {
        lock(&root)?;
    }
    Ok(root)
}

/// Pins the project's versions. It has no dependencies, so no network is used.
fn lock(root: &PathBuf) -> Result<(), String> {
    let status = Command::new("cargo")
        .args(["generate-lockfile", "--offline"])
        .current_dir(root)
        .status()
        .map_err(|error| error.to_string())?;
    if !status.success() {
        return Err("cargo generate-lockfile failed".to_string());
    }
    Ok(())
}
