// Purpose: makes a second install of the Rust compiler for a test, with a
//          home folder of its own: a small program that runs the real
//          compiler from that folder, with the real compiler's libraries
//          linked under it.
// Never:   copies or changes the real compiler.
use std::os::unix::fs::PermissionsExt;
use std::path::{
    Path,
    PathBuf, //
};
use std::process::Command;

const EXECUTABLE: u32 = 0o755;

/// The folder the real compiler is installed in.
fn real_home() -> Result<String, String> {
    let output = Command::new("rustc").args(["--print", "sysroot"]).output();
    let output = output.map_err(|error| error.to_string())?;
    let printed = String::from_utf8_lossy(&output.stdout);
    Ok(printed.trim().to_string())
}

/// Makes the second install in `folder` and returns its compiler program.
pub fn install(folder: &Path) -> Result<PathBuf, String> {
    let real = real_home()?;
    std::fs::create_dir_all(folder.join("bin")).map_err(|error| error.to_string())?;
    let libraries = format!("{real}/lib");
    let linked = std::os::unix::fs::symlink(libraries, folder.join("lib"));
    linked.map_err(|error| error.to_string())?;
    let program = folder.join("bin/rustc");
    let own_home = folder.display();
    let script = format!("#!/bin/sh\nexec \"{real}/bin/rustc\" --sysroot \"{own_home}\" \"$@\"\n");
    std::fs::write(&program, script).map_err(|error| error.to_string())?;
    let permissions = std::fs::Permissions::from_mode(EXECUTABLE);
    std::fs::set_permissions(&program, permissions).map_err(|error| error.to_string())?;
    Ok(program)
}
