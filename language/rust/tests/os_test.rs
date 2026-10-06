// Purpose: the OS build names the system exactly, not only its family.
// Never:   accepts an empty or family-only OS name.
use language_rust::os::os_build;
use std::path::Path;

#[test]
fn the_os_build_names_the_system_and_its_exact_build() -> Result<(), String> {
    let found = os_build(Path::new(env!("CARGO_TARGET_TMPDIR"))).map_err(|b| b.to_string())?;
    let mac = found.starts_with("macOS ") && found.ends_with(')');
    let linux = found.starts_with("Linux ") && found.split(' ').count() == 2;
    assert!(mac || linux, "unexpected OS build: {found}");
    Ok(())
}
