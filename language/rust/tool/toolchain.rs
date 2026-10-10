// Purpose: records the exact Rust compiler the project builds with.
// Never:   reports the machine's default compiler when the project pins another.
use crate::blocked::Blocked;
use crate::digest::digest_of;
use crate::record::Toolchain;
use crate::run::printed;
use std::path::Path;

/// Asks the compiler that `root` selects for its version, host, and binary.
///
/// Running inside the project lets rustup honour a `rust-toolchain.toml`
/// there, so the recorded compiler is the one the build really uses.
pub fn toolchain(root: &Path) -> Result<Toolchain, Blocked> {
    let verbose = printed("rustc", &["-vV"], root)?;
    let version = field(&verbose, "rustc ").ok_or_else(|| unreadable("no version line"))?;
    let host = field(&verbose, "host: ").ok_or_else(|| unreadable("no host line"))?;
    let sysroot = printed("rustc", &["--print", "sysroot"], root)?;
    let compiler = digest_of(&Path::new(sysroot.trim()).join("bin").join("rustc"))?;
    Ok(Toolchain {
        version,
        host,
        compiler,
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
