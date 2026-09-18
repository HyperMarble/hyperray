// Compile the subject and requirement independently before linking the native checker.
// Compiler errors must not trigger handwritten behavior replacements.
use super::{cargo, compile_file, step, Request};
use std::process::Command;

pub fn libraries(request: &Request) -> Result<(), String> {
    match &request.cargo {
        Some(options) => cargo::libraries(request, options),
        None => compile_file::libraries(request),
    }
}

pub fn checker(request: &Request) -> Result<(), String> {
    let mut spin = Command::new(&request.tools.spin);
    spin.args(["-a", "search.pml"]);
    step::execute(&mut spin, &request.directory, "spin")?;
    let mut search = Command::new(&request.tools.clang);
    search
        .args(["-O2", "-DNOREDUCE", "-DSAFETY"])
        .arg(format!("-DMEMLIM={}", request.limits.state_memory_mib))
        .args(["pan.c", "observe.c", "libbinding.a", "-o", "search"]);
    step::execute(&mut search, &request.directory, "search")?;
    let mut replay = Command::new(&request.tools.clang);
    replay.args([
        "-O2",
        "-Wall",
        "-Wextra",
        "-Werror",
        "replay.c",
        "observe.c",
        "libbinding.a",
        "-o",
        "replay",
    ]);
    step::execute(&mut replay, &request.directory, "replay")
}
