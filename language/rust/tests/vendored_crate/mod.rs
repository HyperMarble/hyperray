// Purpose: gives a test project a dependency that comes from outside it: a
//          dynamic library crate kept in the project's `vendor` folder,
//          which Cargo reads in place of crates.io. No network is used.
// Never:   builds anything itself: Cargo does, when the test asks it to.
use std::path::Path;

const MANIFEST: &str = "[package]\nname = \"vendored_shared\"\nversion = \"0.1.0\"\n\
edition = \"2021\"\n\n[lib]\ncrate-type = [\"dylib\"]\n";

const CODE: &str = "pub fn one() -> u8 { 1 }\n";

/// Cargo wants this file beside a vendored crate. It lists no checksums.
const CHECKSUMS: &str = "{\"files\":{},\"package\":null}";

/// Tells Cargo to read crates.io from the project's `vendor` folder.
const SOURCE: &str = "[source.crates-io]\nreplace-with = \"vendored\"\n\n\
[source.vendored]\ndirectory = \"vendor\"\n";

const DEPENDENCY: &str = "\n[dependencies]\nvendored_shared = \"0.1.0\"\n";

/// Writes `text` to `file` inside `root`, making the file's folder first.
fn save(root: &Path, file: &str, text: &str) -> Result<(), String> {
    let path = root.join(file);
    if let Some(folder) = path.parent() {
        std::fs::create_dir_all(folder).map_err(|error| error.to_string())?;
    }
    std::fs::write(&path, text).map_err(|error| error.to_string())
}

/// Puts the crate `vendored_shared` in the project's `vendor` folder, points
/// Cargo at that folder, and makes the project depend on the crate.
pub fn add_to(root: &Path) -> Result<(), String> {
    save(root, "vendor/vendored_shared/Cargo.toml", MANIFEST)?;
    save(root, "vendor/vendored_shared/src/lib.rs", CODE)?;
    save(
        root,
        "vendor/vendored_shared/.cargo-checksum.json",
        CHECKSUMS,
    )?;
    save(root, ".cargo/config.toml", SOURCE)?;
    let manifest = std::fs::read_to_string(root.join("Cargo.toml"));
    let manifest = manifest.map_err(|error| error.to_string())?;
    save(root, "Cargo.toml", &(manifest + DEPENDENCY))
}
