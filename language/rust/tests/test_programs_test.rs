// Purpose: asking for every target builds the project's test programs too,
//          each marked as compiled in test mode; the default build makes none.
// Never:   builds a test program nobody asked for.
mod sample_project;

use language_rust::build_record_with;
use language_rust::choice::{Choice, Targets};
use language_rust::record::{BuildRecord, Outcome};
use sample_project::{create, Sample};

const SOURCE: &str = "pub fn total(a: u8, b: u8) -> u8 { a + b }\n\n#[cfg(test)]\nmod tests {\n    #[test]\n    fn adds() { assert_eq!(super::total(2, 3), 5); }\n}\n";

fn built(name: &str, targets: Targets) -> Result<BuildRecord, String> {
    let root = create(&Sample {
        name,
        file: "lib.rs",
        source: SOURCE,
        locked: true,
    })?;
    let choice = Choice {
        targets,
        ..Choice::default()
    };
    match build_record_with(&root, &choice) {
        Outcome::Built(record) => Ok(*record),
        Outcome::Blocked { reason } => Err(reason),
    }
}

#[test]
fn every_target_includes_the_test_program() -> Result<(), String> {
    let record = built("with_test_program", Targets::All)?;
    let test_programs: Vec<_> = record
        .artifacts
        .iter()
        .filter(|a| a.compiled.test)
        .collect();
    assert_eq!(test_programs.len(), 1, "{:?}", record.artifacts);
    assert!(record.artifacts.iter().any(|a| !a.compiled.test));
    assert_eq!(record.settings.requested.targets, Targets::All);
    Ok(())
}

#[test]
fn the_default_build_makes_no_test_program() -> Result<(), String> {
    let record = built("without_test_program", Targets::Default)?;
    assert!(record.artifacts.iter().all(|a| !a.compiled.test));
    Ok(())
}
