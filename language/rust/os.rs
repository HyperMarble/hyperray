// Purpose: names the exact operating-system build the code was built on.
// Never:   reports a version family where the exact build is available.
use crate::blocked::Blocked;
use crate::run::printed;
use std::path::Path;

/// The OS build, e.g. `macOS 27.0 (25A123)` or `Linux 6.8.0-45-generic`.
///
/// System-call certificates are keyed by this, so a family name such as
/// "macOS 27" would let one build's certificate cover another.
pub fn os_build(folder: &Path) -> Result<String, Blocked> {
    if cfg!(target_os = "macos") {
        let version = printed("sw_vers", &["-productVersion"], folder)?;
        let build = printed("sw_vers", &["-buildVersion"], folder)?;
        return Ok(format!("macOS {} ({})", version.trim(), build.trim()));
    }
    let release = printed("uname", &["-sr"], folder)?;
    Ok(release.trim().to_string())
}
