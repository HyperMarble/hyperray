// Purpose: writes the adapter's own tiny empty crate. Cargo builds it from
//          inside the project's folder when the adapter has a question for
//          the compiler Cargo runs there.
// Never:   writes into the project's own folders.
use crate::blocked::Blocked;
use std::path::{
    Path,
    PathBuf, //
};

const MANIFEST: &str = "[package]\nname = \"hyperray-compiler\"\nversion = \"0.0.0\"\n\
edition = \"2021\"\n\n[lib]\npath = \"lib.rs\"\n\n[workspace]\n";

/// Writes the crate under `folder` and returns its Cargo.toml.
pub fn write(folder: &Path) -> Result<PathBuf, Blocked> {
    let home = folder.join("compiler");
    std::fs::create_dir_all(&home).map_err(|error| unreadable(&home, error))?;
    let code = home.join("lib.rs");
    std::fs::write(&code, "").map_err(|error| unreadable(&code, error))?;
    let manifest = home.join("Cargo.toml");
    std::fs::write(&manifest, MANIFEST).map_err(|error| unreadable(&manifest, error))?;
    Ok(manifest)
}

fn unreadable(path: &Path, error: std::io::Error) -> Blocked {
    Blocked::Unreadable {
        what: path.display().to_string(),
        cause: error.to_string(),
    }
}
