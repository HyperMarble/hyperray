// Purpose: the record's facts about how the code was built reach the JSON:
//          what was asked for, each file's features and settings, the
//          environment, native code, and the C compiler.
// Never:   drops one of those facts on the way to the loader.
mod sample_record;

use language_rust::record::Outcome;

#[test]
fn how_the_code_was_built_reaches_the_json() -> Result<(), String> {
    let outcome = Outcome::Built(Box::new(sample_record::record()));
    let written = serde_json::to_value(&outcome).map_err(|error| error.to_string())?;
    assert_eq!(written["settings"]["requested"]["profile"], "release");
    assert_eq!(
        written["settings"]["requested"]["features"]["kind"],
        "default"
    );
    assert_eq!(written["artifacts"][0]["features"][0], "fancy");
    assert_eq!(written["artifacts"][0]["compiled"]["overflow_checks"], true);
    assert_eq!(written["environment"][0]["name"], "RUSTFLAGS");
    assert_eq!(written["native_code"][0]["linked_libs"][0], "static=zstd");
    assert_eq!(written["c_toolchain"]["sdk_version"], "27.0");
    Ok(())
}
