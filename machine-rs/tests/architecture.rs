// The architecture must be read from the real model file, and an unusable
// file must be named rather than treated as an empty model.
use hyperray_machine::architecture;
use std::path::Path;

#[test]
fn the_model_file_is_read_whole() {
    let Ok(path) = std::env::var("HYPERRAY_ARM64_SAIL_IR") else {
        return;
    };
    let found = architecture::read(Path::new(&path));
    let reason = match &found {
        Err(error) => error.clone(),
        Ok(arch) if arch.bytes.is_empty() => "read no bytes".to_string(),
        Ok(_) => String::new(),
    };
    assert!(reason.is_empty(), "{reason}");
}

#[test]
fn an_absent_model_is_a_reported_failure() {
    let found = architecture::read(Path::new("/nonexistent/armv8p5.ir"));
    assert!(found.is_err(), "absent model returned {found:?}");
}
