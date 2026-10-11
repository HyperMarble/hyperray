// Purpose: makes a tiny real Cargo workspace with one member crate on disk.
// Never:   shares a workspace folder between tests; each name gets its own.
use std::path::PathBuf;
use std::process::Command;

/// Writes the workspace and, when `locked`, pins it with a Cargo.lock at its
/// top. Returns the top folder and the member crate's folder.
pub fn create(name: &str, locked: bool) -> Result<(PathBuf, PathBuf), String> {
    let top = PathBuf::from(env!("CARGO_TARGET_TMPDIR")).join(name);
    if top.exists() {
        std::fs::remove_dir_all(&top).map_err(|error| error.to_string())?;
    }
    let member = top.join("member");
    std::fs::create_dir_all(member.join("src")).map_err(|error| error.to_string())?;
    let workspace = "[workspace]\nmembers = [\"member\"]\nresolver = \"2\"\n";
    std::fs::write(top.join("Cargo.toml"), workspace).map_err(|error| error.to_string())?;
    let manifest =
        format!("[package]\nname = \"{name}_member\"\nversion = \"0.1.0\"\nedition = \"2021\"\n");
    std::fs::write(member.join("Cargo.toml"), manifest).map_err(|error| error.to_string())?;
    std::fs::write(
        member.join("src").join("lib.rs"),
        "pub fn one() -> u8 { 1 }\n",
    )
    .map_err(|error| error.to_string())?;
    if locked {
        lock(&top)?;
    }
    Ok((top, member))
}

/// Pins the workspace's versions. It has no dependencies, so no network is used.
fn lock(top: &PathBuf) -> Result<(), String> {
    let status = Command::new("cargo")
        .args(["generate-lockfile", "--offline"])
        .current_dir(top)
        .status()
        .map_err(|error| error.to_string())?;
    if !status.success() {
        return Err("cargo generate-lockfile failed".to_string());
    }
    Ok(())
}
