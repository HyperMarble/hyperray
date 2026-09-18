// Supply a build-time constant so successful linking depends on Cargo's build script.
// This script must not examine the checker or execute the subject.
use std::{env, fs, path::PathBuf};

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let directory = PathBuf::from(env::var("OUT_DIR")?);
    fs::write(directory.join("increment.rs"), "const BASE: u64 = 5;\n")?;
    println!("cargo:rerun-if-changed=build.rs");
    Ok(())
}
