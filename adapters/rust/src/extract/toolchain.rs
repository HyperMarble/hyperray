// The rustc executable that matches a rustc driver. A driver cannot safely
// load compiler internals from one toolchain and reuse another toolchain.

use std::path::{Path, PathBuf};
use std::process::Command;

pub fn compiler_for(driver: &Path) -> Result<PathBuf, String> {
    let output = Command::new(driver)
        .args(["--print", "sysroot"])
        .output()
        .map_err(|error| format!("cannot run {}: {error}", driver.display()))?;
    if !output.status.success() {
        return Err(String::from_utf8_lossy(&output.stderr).into_owned());
    }
    let text = String::from_utf8(output.stdout).map_err(|error| error.to_string())?;
    let compiler = PathBuf::from(text.trim()).join("bin").join("rustc");
    match compiler.is_file() {
        true => Ok(compiler),
        false => Err(format!("{} is not a compiler", compiler.display())),
    }
}
