// The CBMC decoder must preserve all reported loops and source fields.
// It must reject output that has no single, unambiguous loop collection.

use hyperray_rust::bound::{decode_cbmc_loop_inventory, Error};

const INVENTORY: &str = r#"[
  {"messageText":"status","messageType":"STATUS-MESSAGE"},
  {"loops":[
    {"name":"crate::first.0","sourceLocation":{
      "file":"src/lib.rs","function":"crate::first","line":"7",
      "column":"3","pragma":["checked:bounds-check"],
      "workingDirectory":"/work"}},
    {"name":"__rust_alloc.0","sourceLocation":{
      "file":"kani_lib.c","function":"__rust_alloc","line":"43"}}
  ]}
]"#;

#[test]
fn every_cbmc_loop_and_location_survives() -> Result<(), Box<dyn std::error::Error>> {
    let loops = decode_cbmc_loop_inventory("fixture", INVENTORY.as_bytes())?;
    let first = loops
        .first()
        .ok_or_else(|| std::io::Error::other("first loop is missing"))?;
    let location = first
        .source_location
        .as_ref()
        .ok_or_else(|| std::io::Error::other("first loop location is missing"))?;
    assert_eq!(loops.len(), 2);
    assert_eq!(first.name, "crate::first.0");
    assert_eq!(location.file.as_deref(), Some("src/lib.rs"));
    assert_eq!(location.function.as_deref(), Some("crate::first"));
    assert_eq!(location.line.as_deref(), Some("7"));
    assert_eq!(location.column.as_deref(), Some("3"));
    assert_eq!(location.pragmas, ["checked:bounds-check"]);
    assert_eq!(location.working_directory.as_deref(), Some("/work"));
    assert_eq!(loops[1].name, "__rust_alloc.0");
    Ok(())
}

#[test]
fn missing_or_duplicate_inventory_is_an_error() {
    let missing = Error::Model {
        reason: "fixture: CBMC returned no loop inventory".to_string(),
    };
    let duplicate = Error::Model {
        reason: "fixture: CBMC returned more than one loop inventory".to_string(),
    };
    assert_eq!(decode_cbmc_loop_inventory("fixture", b"[]"), Err(missing));
    assert_eq!(
        decode_cbmc_loop_inventory("fixture", br#"[{"loops":[]},{"loops":[]}]"#),
        Err(duplicate)
    );
}
