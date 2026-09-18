// A successful tool process without compiler output is not an inventory.
// Existing dumps must not rescue a missing current compiler invocation.
use hyperray_rust::extract;
use std::path::PathBuf;

#[test]
#[ignore = "requires HYPERRAY_MIR_DUMP and HYPERRAY_COMPILER_TEST_OUTPUT"]
fn successful_process_without_inventory_is_an_error() -> Result<(), Box<dyn std::error::Error>> {
    let mut request = super::request::request()?;
    request.driver = PathBuf::from(std::env::var("HYPERRAY_MIR_DUMP")?);
    request.cargo = PathBuf::from(std::env::var("CARGO")?);
    request.output_directory =
        PathBuf::from(std::env::var("HYPERRAY_COMPILER_TEST_OUTPUT")?).join("missing-inventory");
    let first = extract::run(&request);
    super::evidence::inventory(&first, Ok("neither"))?;
    request.cargo = PathBuf::from("/usr/bin/true");
    let missing = extract::run(&request);
    assert_eq!(missing.exit_code, None);
    assert_eq!(missing.dumps, 0);
    assert!(missing.log.contains("without fresh compiler inventory"));
    Ok(())
}
