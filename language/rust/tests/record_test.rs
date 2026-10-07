// Purpose: the build record's JSON shape is what the loader will read.
// Never:   lets a blocked outcome look like a build.
mod sample_record;

use language_rust::record::Outcome;
use serde_json::json;

#[test]
fn a_blocked_outcome_names_its_status_and_reason() -> Result<(), String> {
    let outcome = Outcome::Blocked {
        reason: "no Cargo.toml".to_string(),
    };
    let written = serde_json::to_value(&outcome).map_err(|error| error.to_string())?;
    assert_eq!(
        written,
        json!({"status": "blocked", "reason": "no Cargo.toml"})
    );
    Ok(())
}

#[test]
fn a_built_outcome_carries_every_part_of_the_record() -> Result<(), String> {
    let outcome = Outcome::Built(Box::new(sample_record::record()));
    let written = serde_json::to_value(&outcome).map_err(|error| error.to_string())?;
    assert_eq!(written["status"], "built");
    assert_eq!(written["artifacts"][0]["kind"], "lib");
    assert_eq!(written["toolchain"]["host"], "aarch64-apple-darwin");
    assert_eq!(written["settings"]["lock_file"]["path"], "Cargo.lock");
    assert_eq!(written["os_build"], "macOS 27.0 (25A123)");
    Ok(())
}
