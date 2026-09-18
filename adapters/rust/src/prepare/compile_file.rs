// Standalone source files retain the original compiler preparation path.
// This path must not guess or reconstruct Cargo dependency graphs.
use super::{step, FunctionSource, Request};
use std::process::Command;

pub fn libraries(request: &Request) -> Result<(), String> {
    library(request, &request.subject, "subject")?;
    library(request, &request.requirement, "requirement")?;
    let mut binding = Command::new(&request.tools.rustc);
    binding
        .args([
            "binding.rs",
            "--crate-type",
            "staticlib",
            "--edition",
            "2021",
            "-D",
            "warnings",
        ])
        .args(["-C", "panic=abort", "--extern", "subject=libsubject.rlib"])
        .args([
            "--extern",
            "requirement=librequirement.rlib",
            "-o",
            "libbinding.a",
        ]);
    step::execute(&mut binding, &request.directory, "binding")
}

fn library(request: &Request, source: &FunctionSource, name: &str) -> Result<(), String> {
    let mut command = Command::new(&request.tools.rustc);
    command
        .arg(&source.source)
        .args(["--crate-name", name, "--crate-type", "rlib"])
        .args([
            "--edition",
            "2021",
            "-D",
            "warnings",
            "-C",
            "panic=abort",
            "-C",
        ])
        .arg(format!("opt-level={}", request.optimization))
        .args(["-o", &format!("lib{name}.rlib")]);
    step::execute(&mut command, &request.directory, name)
}
