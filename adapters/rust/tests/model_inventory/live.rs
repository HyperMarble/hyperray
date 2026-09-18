// A real Kani crate proves that both harness modes yield CBMC-readable models.
// An unset fixture path is reported as a skipped external integration test.

use crate::support;
use hyperray_rust::bound::{HarnessMode, Model};

#[test]
fn declared_and_automatic_harnesses_build_final_models() -> Result<(), Box<dyn std::error::Error>> {
    let Some(config) = support::configured() else {
        eprintln!("skipped: HYPERRAY_KANI_CRATE is not set");
        return Ok(());
    };
    let declared = support::build(&config, HarnessMode::Declared)?;
    validate(&declared);
    assert!(declared.automatic_selections.is_empty());

    let automatic = support::build(&config, HarnessMode::Automatic)?;
    validate(&automatic);
    assert!(!automatic.automatic_selections.is_empty());

    let json = serde_json::to_vec(&automatic)?;
    let decoded: Model = serde_json::from_slice(&json)?;
    assert_eq!(decoded, automatic);
    Ok(())
}

fn validate(model: &Model) {
    assert!(!model.harnesses.is_empty());
    assert!(model.rows.is_empty());
    assert!(!model.tools.kani.is_empty());
    assert!(!model.tools.cbmc.is_empty());
    assert!(model.artifact_dir.is_dir());
    assert!(model
        .harnesses
        .iter()
        .all(|harness| harness.goto_file.is_file()));
}
