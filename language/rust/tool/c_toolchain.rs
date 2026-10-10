// Purpose: records the C compiler that build scripts use by default when they
//          compile C code into the program, and on macOS the Apple SDK, as the
//          system's own tools name them.
// Never:   guesses a compiler: the tool answers, or the build is blocked. A
//          compiler chosen through `CC` is in the environment record instead.
use crate::blocked::Blocked;
use crate::build_facts::CToolchain;
use crate::digest::digest_of;
use crate::run::printed;
use std::path::Path;

/// The default C compiler and SDK for builds run in `folder`.
pub fn c_toolchain(folder: &Path) -> Result<CToolchain, Blocked> {
    if cfg!(target_os = "macos") {
        return apple(folder);
    }
    let path = printed("sh", &["-c", "command -v cc"], folder)?;
    let version = printed("cc", &["--version"], folder)?;
    Ok(CToolchain {
        compiler: digest_of(Path::new(path.trim()))?,
        version: first_line(&version),
        sdk_path: None,
        sdk_version: None,
    })
}

/// On macOS, Apple's `xcrun` names the compiler and SDK that `cc` runs.
fn apple(folder: &Path) -> Result<CToolchain, Blocked> {
    let path = printed("xcrun", &["--find", "clang"], folder)?;
    let version = printed("xcrun", &["clang", "--version"], folder)?;
    let sdk_path = printed("xcrun", &["--show-sdk-path"], folder)?;
    let sdk_version = printed("xcrun", &["--show-sdk-version"], folder)?;
    Ok(CToolchain {
        compiler: digest_of(Path::new(path.trim()))?,
        version: first_line(&version),
        sdk_path: Some(sdk_path.trim().to_string()),
        sdk_version: Some(sdk_version.trim().to_string()),
    })
}

fn first_line(text: &str) -> String {
    text.lines().next().unwrap_or_default().trim().to_string()
}
