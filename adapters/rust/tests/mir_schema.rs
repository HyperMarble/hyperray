// The stable adapter accepts only the compiler schema that it implements.
// A stale schema must never become an empty analysis.

use hyperray_rust::mir::{read, Dump, SCHEMA_VERSION};

#[test]
fn the_current_schema_completes_a_json_round_trip() -> Result<(), Box<dyn std::error::Error>> {
    let dump = Dump {
        schema_version: SCHEMA_VERSION,
        crate_name: "sample".to_string(),
        items: Vec::new(),
        instances: Vec::new(),
    };
    let json = serde_json::to_vec(&dump)?;
    let decoded: Dump = serde_json::from_slice(&json)?;
    assert_eq!(decoded, dump);
    Ok(())
}

#[test]
fn schema_one_returns_the_exact_schema_error() -> Result<(), Box<dyn std::error::Error>> {
    let path =
        std::env::temp_dir().join(format!("hyperray-mir-schema-{}.json", std::process::id()));
    std::fs::write(
        &path,
        br#"{"schema_version":1,"crate_name":"sample","items":[]}"#,
    )?;
    let result = read(std::fs::File::open(&path)?);
    std::fs::remove_file(path)?;
    let error = result.err().ok_or("schema 1 was accepted")?;
    assert_eq!(error.to_string(), "MIR schema 1 is not schema 4");
    Ok(())
}
