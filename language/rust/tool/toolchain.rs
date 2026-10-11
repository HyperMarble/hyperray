// Purpose: records the exact Rust compiler the project builds with.
// Never:   reports the machine's default compiler when the project pins
//          another, or when the owner tells Cargo to run another.
use crate::blocked::Blocked;
use crate::digest::digest_of;
use crate::record::Toolchain;
use crate::run::printed;
use std::path::Path;

/// Asks the compiler installed in `home` for its version and host, and
/// names its program by content.
///
/// `home` is the folder the compiler Cargo runs for the project says it is
/// installed in, so the recorded compiler is the one the build really uses:
/// the one a `rust-toolchain.toml` pins, or `RUSTC` or Cargo's settings name.
pub fn toolchain(home: &Path, root: &Path) -> Result<Toolchain, Blocked> {
    let program = home.join("bin").join("rustc");
    let program_name = program.display().to_string();
    let verbose = printed(&program_name, &["-vV"], root)?;
    let version = field(&verbose, "rustc ").ok_or_else(|| unreadable("no version line"))?;
    let host = field(&verbose, "host: ").ok_or_else(|| unreadable("no host line"))?;
    Ok(Toolchain {
        version,
        host,
        compiler: digest_of(&program)?,
    })
}

/// `rustc -vV` printed something without the line this adapter needs.
fn unreadable(cause: &str) -> Blocked {
    Blocked::Unreadable {
        what: "rustc -vV".to_string(),
        cause: cause.to_string(),
    }
}

/// The value after `name` on the line that starts with it.
fn field(text: &str, name: &str) -> Option<String> {
    text.lines()
        .find_map(|line| line.strip_prefix(name))
        .map(str::to_string)
}
