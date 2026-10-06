// Purpose: the recorded compiler is a real file, named by its content.
// Never:   accepts a toolchain record with an empty version or host.
use language_rust::toolchain::toolchain;
use std::path::Path;

#[test]
fn the_compiler_is_recorded_with_version_host_and_hash() -> Result<(), String> {
    let found =
        toolchain(Path::new(env!("CARGO_TARGET_TMPDIR"))).map_err(|blocked| blocked.to_string())?;
    assert!(found
        .version
        .starts_with(|first: char| first.is_ascii_digit()));
    assert!(!found.host.is_empty());
    assert!(Path::new(&found.compiler.path).is_file());
    assert_eq!(found.compiler.sha256.len(), 64);
    Ok(())
}
