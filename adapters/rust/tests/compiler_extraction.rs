// Live compiler tests use the public adapter API, without loop-limit rules.
// Missing compiler configuration is an error when explicitly selected.

use hyperray_rust::{extract, mir};
use std::collections::BTreeSet;
use std::path::PathBuf;

#[test]
#[ignore = "requires HYPERRAY_MIR_DUMP and HYPERRAY_COMPILER_TEST_OUTPUT"]
fn compiler_extracts_each_declared_function() -> Result<(), Box<dyn std::error::Error>> {
    let result = compile("compiler-extraction")?;
    assert_eq!(result.exit_code, Some(0), "{}", result.log);
    assert!(!result.dump_paths.is_empty(), "{}", result.log);
    let mut names = BTreeSet::new();
    for path in result.dump_paths {
        let dump = mir::read(std::fs::File::open(path)?)?;
        names.extend(
            dump.items
                .into_iter()
                .filter_map(|item| item.body.map(|body| (item.name, body.blocks.is_empty()))),
        );
    }
    for name in ["counted", "iterator", "recursive", "future", "generic"] {
        let expected = (format!("compiler_extraction::{name}"), false);
        assert!(names.contains(&expected), "missing {expected:?}: {names:?}");
    }
    Ok(())
}

#[test]
#[ignore = "requires HYPERRAY_MIR_DUMP and HYPERRAY_COMPILER_TEST_OUTPUT"]
fn compiler_errors_remain_errors() -> Result<(), Box<dyn std::error::Error>> {
    let result = compile("compiler-rejection")?;
    assert!(result.exit_code.is_some(), "{}", result.log);
    assert_ne!(result.exit_code, Some(0), "{}", result.log);
    assert!(
        result.log.contains("compiler rejection fixture"),
        "{}",
        result.log
    );
    assert_eq!(result.dumps, 0);
    Ok(())
}

fn compile(fixture: &str) -> Result<extract::Run, Box<dyn std::error::Error>> {
    let driver = PathBuf::from(std::env::var("HYPERRAY_MIR_DUMP")?);
    let output = PathBuf::from(std::env::var("HYPERRAY_COMPILER_TEST_OUTPUT")?);
    let source = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("../../fixtures/rust")
        .join(fixture);
    let request = extract::CompileRequest {
        driver,
        cargo: PathBuf::from(std::env::var("CARGO")?),
        crate_directory: source,
        output_directory: output.join(fixture),
        features: hyperray_rust::FeatureSelection {
            default_features: true,
            features: Vec::new(),
        },
    };
    Ok(extract::run(&request))
}
