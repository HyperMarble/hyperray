// Purpose: the C compiler build scripts use is recorded as a real file named
//          by its content, with its version, and on macOS the SDK.
// Never:   records a compiler that is not a file on this machine.
use language_rust::c_toolchain::c_toolchain;
use std::path::Path;

#[test]
fn the_c_compiler_is_a_real_file_with_its_version() -> Result<(), String> {
    let found = c_toolchain(Path::new(env!("CARGO_TARGET_TMPDIR"))).map_err(|b| b.to_string())?;
    assert!(Path::new(&found.compiler.path).is_file());
    assert_eq!(found.compiler.sha256.len(), 64);
    assert!(!found.version.is_empty());
    if cfg!(target_os = "macos") {
        assert!(found.sdk_version.as_deref().is_some_and(|v| !v.is_empty()));
    }
    Ok(())
}
