// Public request tests do not execute the placeholder compiler tool.
// Invalid paths and missing feature policy must remain input errors.
use hyperray_rust::{
    extract::{self, CompileRequest},
    FeatureSelection,
};
use std::path::PathBuf;

pub fn request() -> Result<CompileRequest, Box<dyn std::error::Error>> {
    Ok(CompileRequest {
        driver: std::env::current_exe()?,
        cargo: std::env::current_exe()?,
        crate_directory: PathBuf::from(env!("CARGO_MANIFEST_DIR"))
            .join("../../fixtures/rust/compiler-feature-selection"),
        output_directory: std::env::temp_dir(),
        features: FeatureSelection {
            default_features: false,
            features: Vec::new(),
        },
    })
}

#[test]
fn public_request_roundtrip() -> Result<(), Box<dyn std::error::Error>> {
    let request = request()?;
    request.validate()?;
    let encoded = serde_json::to_value(&request)?;
    let decoded: CompileRequest = serde_json::from_value(encoded.clone())?;
    assert_eq!(serde_json::to_value(decoded)?, encoded);
    Ok(())
}

#[test]
fn invalid_requests_remain_errors() -> Result<(), Box<dyn std::error::Error>> {
    let original = request()?;
    let mut invalid = original.clone();
    invalid.features.features.push("left,right".into());
    assert!(invalid.validate().is_err());
    invalid = original.clone();
    invalid.cargo = "cargo".into();
    assert!(invalid.validate().is_err());
    invalid = original.clone();
    invalid.output_directory = "relative".into();
    assert!(invalid.validate().is_err());
    let failed = extract::run(&invalid);
    assert_eq!(failed.exit_code, None);
    assert!(!failed.log.is_empty());
    assert_eq!(failed.features, invalid.features);
    let mut encoded = serde_json::to_value(original)?;
    let removed = encoded
        .as_object_mut()
        .ok_or("request is not an object")?
        .remove("features");
    assert!(removed.is_some());
    assert!(serde_json::from_value::<CompileRequest>(encoded).is_err());
    Ok(())
}
