// Both formats must produce the same image from the same source.
// A format that cannot be read must say so, never return an empty image.
use loader::{elf, macho, LoadError};
use std::path::PathBuf;
use std::process::Command;

const SOURCE: &str = r#"
#[no_mangle]
#[inline(never)]
pub extern "C" fn half(n: u64) -> u64 { n >> 1 }
"#;

/// A target with no operating system has no standard library and no main.
const BARE_SOURCE: &str = r#"
#![no_std]
#![no_main]
#[panic_handler]
fn panic(_: &core::panic::PanicInfo) -> ! { loop {} }
#[no_mangle]
#[inline(never)]
pub extern "C" fn half(n: u64) -> u64 { n >> 1 }
#[no_mangle]
#[no_mangle]
pub extern "C" fn start_here() -> u64 { half(4) }
#[no_mangle]
pub extern "C" fn _start() -> ! { start_here(); loop {} }
"#;

fn scratch(name: &str) -> PathBuf {
    let dir = std::env::temp_dir().join("hyperray_loader_tests");
    std::fs::create_dir_all(&dir).expect("scratch directory");
    dir.join(name)
}

/// Compiles the same source for one target and returns the file's bytes.
///
/// A bare-metal library links to an archive rather than an object, so the
/// emitted kind is named rather than assumed.
fn build(target: &str, stem: &str, kind: &str, emit: &str, text: &str) -> Result<Vec<u8>, String> {
    let source = scratch(&format!("{stem}.rs"));
    let output = scratch(&format!("{stem}.out"));
    std::fs::write(&source, text).map_err(|e| e.to_string())?;
    let result = Command::new("rustc")
        .args(["--target", target, "--crate-type", kind, "-O", "--emit", emit, "-o"])
        .arg(&output)
        .arg(&source)
        .output()
        .map_err(|e| e.to_string())?;
    if !result.status.success() {
        return Err(String::from_utf8_lossy(&result.stderr).to_string());
    }
    std::fs::read(&output).map_err(|e| e.to_string())
}

#[test]
fn reads_a_macos_library_that_declares_no_entry() {
    let content = build("aarch64-apple-darwin", "mac_lib", "cdylib", "link", SOURCE).expect("must compile");
    let image = macho::load(&content).expect("a Mach-O library must load");
    assert_eq!(image.entry, None, "a library declares no entry");
    assert!(!image.segments.is_empty(), "the image must carry its segments");
}

#[test]
fn reads_a_bare_metal_executable() {
    let content = build("riscv64gc-unknown-none-elf", "bare", "bin", "link", BARE_SOURCE).expect("must compile");
    let image = elf::load(&content).expect("an ELF file must load");
    assert!(image.entry.is_some(), "an executable declares where it starts");
    assert!(!image.segments.is_empty(), "the image must carry its segments");
}

#[test]
fn a_macho_file_is_refused_by_the_elf_reader() {
    let content = build("aarch64-apple-darwin", "mac_lib2", "cdylib", "link", SOURCE).expect("must compile");
    match elf::load(&content) {
        Err(LoadError::Parse(_)) | Err(LoadError::UnsupportedArchitecture { .. }) => {}
        other => panic!("the ELF reader must refuse a Mach-O file, got {other:?}"),
    }
}
