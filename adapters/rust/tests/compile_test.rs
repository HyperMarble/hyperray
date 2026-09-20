// The adapter must compile real source and reject what it cannot compile.
// A failure must name the cause, never return a path to a missing file.
use super::compile::{compile, CompileError, BARE_METAL_RISCV};
use std::path::PathBuf;

const SOURCE: &str = r#"
#![no_std]
#![no_main]
#[panic_handler]
fn panic(_: &core::panic::PanicInfo) -> ! { loop {} }
#[no_mangle]
pub extern "C" fn half(n: u64) -> u64 { n >> 1 }
#[no_mangle]
pub extern "C" fn _start() -> ! { let _ = half(4); loop {} }
"#;

fn scratch(name: &str) -> PathBuf {
    let dir = std::env::temp_dir().join("hyperray_rust_adapter");
    std::fs::create_dir_all(&dir).expect("scratch directory");
    dir.join(name)
}

#[test]
fn compiles_real_source_for_a_target_without_an_operating_system() {
    let source = scratch("half.rs");
    let output = scratch("half.bin");
    std::fs::write(&source, SOURCE).expect("write source");
    let compiled = compile(&source, &output, BARE_METAL_RISCV).expect("source must compile");
    assert_eq!(compiled.target, BARE_METAL_RISCV);
    assert!(compiled.binary.is_file(), "the reported binary must exist");
}

#[test]
fn absent_source_is_reported_by_path() {
    let missing = scratch("not-written.rs");
    let _ = std::fs::remove_file(&missing);
    match compile(&missing, &scratch("unused.bin"), BARE_METAL_RISCV) {
        Err(CompileError::SourceMissing(path)) => assert_eq!(path, missing),
        other => panic!("absent source must be named, got {other:?}"),
    }
}

#[test]
fn rejected_source_carries_the_compiler_output() {
    let source = scratch("broken.rs");
    std::fs::write(&source, "fn main() { this is not rust }").expect("write source");
    match compile(&source, &scratch("broken.bin"), BARE_METAL_RISCV) {
        Err(CompileError::Rejected { output, .. }) => {
            assert!(!output.is_empty(), "the compiler's own message must be carried out")
        }
        other => panic!("broken source must be rejected, got {other:?}"),
    }
}
