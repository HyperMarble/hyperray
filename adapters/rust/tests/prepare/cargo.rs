// External callers must construct Cargo options without hidden defaults.
// Feature mistakes remain request errors, not silent substitutions.
use hyperray_rust::prepare::{CargoOptions, FeatureSelection, Request};

#[test]
fn public_cargo_request() -> Result<(), Box<dyn std::error::Error>> {
    let mut request = super::fixture::request()?;
    let root = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../fixtures/rust/cargo-preparation");
    request.subject.source = root.join("subject/Cargo.toml");
    request.requirement.source = root.join("requirement/Cargo.toml");
    request.cargo = Some(CargoOptions {
        executable: std::env::current_exe()?,
        subject: FeatureSelection {
            default_features: false,
            features: vec!["boost".into()],
        },
        requirement: FeatureSelection {
            default_features: true,
            features: Vec::new(),
        },
    });
    request.validate()?;
    let encoded = serde_json::to_value(&request)?;
    let decoded = serde_json::from_value::<Request>(encoded.clone())?;
    assert_eq!(serde_json::to_value(decoded)?, encoded);
    request.subject.source = super::fixture::request()?.subject.source;
    assert!(request.validate().is_err());
    Ok(())
}

#[test]
fn reject_ambiguous_features() {
    for feature in ["", "boost,broken", "boost broken", "boost\nbroken"] {
        let selection = FeatureSelection {
            default_features: false,
            features: vec![feature.into()],
        };
        assert!(selection.validate().is_err(), "{feature:?}");
    }
}

#[test]
fn reject_missing_feature_policy() {
    for value in [
        serde_json::json!({"features":[]}),
        serde_json::json!({"default_features":true}),
        serde_json::json!({"default_features":true,"features":[],"guess":true}),
    ] {
        assert!(serde_json::from_value::<FeatureSelection>(value).is_err());
    }
}
