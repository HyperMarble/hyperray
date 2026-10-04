// Purpose: compiles Rust source to a binary for proving.
// Never:   runs the compiled program, or reads a value from running it.
// In:      a source path, an output path, a target
// Out:     the binary that was produced, and the target it was built for
// Fails:   the source is absent, the compiler is absent, the source is rejected
use std::path::{Path, PathBuf};
use std::process::Command;

/// Targets with no operating system underneath the compiled code.
///
/// Bare metal and std emit identical instructions for the same function, so
/// choosing one of these does not change what is proved. It removes the
/// dynamic loader and the operating-system calls that surround the code.
pub const BARE_METAL_RISCV: &str = "riscv64gc-unknown-none-elf";
pub const BARE_METAL_ARM64: &str = "aarch64-unknown-none";

/// A target whose operating-system calls are declared in the compiled file.
///
/// A program that reads a file declares each call it can make, so the calls
/// needing a contract are named rather than found by searching.
pub const DECLARED_OS_CALLS: &str = "wasm32-wasip1";

#[derive(Debug, PartialEq)]
pub enum CompileError {
    SourceMissing(PathBuf),
    CompilerAbsent(String),
    Rejected { status: Option<i32>, output: String },
    OutputMissing(PathBuf),
}

#[derive(Debug, PartialEq)]
pub struct Compiled {
    pub binary: PathBuf,
    pub target: String,
}

/// Compiles one source file and returns the binary it produced.
pub fn compile(source: &Path, output: &Path, target: &str) -> Result<Compiled, CompileError> {
    if !source.is_file() {
        return Err(CompileError::SourceMissing(source.to_path_buf()));
    }
    let result = Command::new("rustc")
        .args(["--target", target, "-O", "-o"])
        .arg(output)
        .arg(source)
        .output()
        .map_err(|error| CompileError::CompilerAbsent(error.to_string()))?;
    if !result.status.success() {
        return Err(CompileError::Rejected {
            status: result.status.code(),
            output: String::from_utf8_lossy(&result.stderr).to_string(),
        });
    }
    if !output.is_file() {
        return Err(CompileError::OutputMissing(output.to_path_buf()));
    }
    Ok(Compiled { binary: output.to_path_buf(), target: target.to_string() })
}
