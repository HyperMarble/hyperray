// Purpose: the recorded compiler is a real file, named by its content.
// Never:   accepts a toolchain record with an empty version or host.
use language_rust::run::printed;
use language_rust::toolchain::toolchain;
use std::path::Path;

#[test]
fn the_compiler_is_recorded_with_version_host_and_hash() -> Result<(), String> {
    let folder = Path::new(env!("CARGO_TARGET_TMPDIR"));
    let home = printed("rustc", &["--print", "sysroot"], folder);
    let home = home.map_err(|blocked| blocked.to_string())?;
    let found = toolchain(Path::new(home.trim()), folder);
    let found = found.map_err(|blocked| blocked.to_string())?;
    assert!(found
        .version
        .starts_with(|first: char| first.is_ascii_digit()));
    assert!(!found.host.is_empty());
    assert!(Path::new(&found.compiler.path).is_file());
    assert_eq!(found.compiler.sha256.len(), 64);
    Ok(())
}
