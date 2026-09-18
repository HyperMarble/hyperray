// Exercise feature choices through the real compiler driver and public API.
// Missing tools are errors when this explicitly ignored integration is selected.
use hyperray_rust::extract;
use std::collections::BTreeSet;
use std::path::PathBuf;

#[test]
#[ignore = "requires HYPERRAY_MIR_DUMP and HYPERRAY_COMPILER_TEST_OUTPUT"]
fn compiler_obeys_each_selection() -> Result<(), Box<dyn std::error::Error>> {
    let mut request = super::request::request()?;
    request.driver = PathBuf::from(std::env::var("HYPERRAY_MIR_DUMP")?);
    request.cargo = PathBuf::from(std::env::var("CARGO")?);
    request.output_directory =
        PathBuf::from(std::env::var("HYPERRAY_COMPILER_TEST_OUTPUT")?).join("feature-selections");
    let before = super::evidence::snapshot(&request.crate_directory)?;
    let mut paths = BTreeSet::new();
    let cases = super::cases::cases();
    for case in &cases {
        request.features = case.features.clone();
        let result = extract::run(&request);
        assert_eq!(result.features, request.features);
        super::evidence::inventory(&result, case.expected)?;
        for path in result.dump_paths {
            assert!(paths.insert(path.clone()), "reused stale dump: {path:?}");
        }
        println!("COMPILER_SELECTION case={} passed", case.name);
    }
    assert_eq!(before, super::evidence::snapshot(&request.crate_directory)?);
    println!(
        "COMPILER_SELECTION completed={} fresh_dumps={}",
        cases.len(),
        paths.len()
    );
    Ok(())
}
