// Share explicit Cargo feature selection across compilation entry points.
// Invalid feature lists must not become guessed defaults.
use serde::{Deserialize, Serialize};

#[derive(Clone, Debug, Deserialize, Serialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct FeatureSelection {
    pub default_features: bool,
    pub features: Vec<String>,
}

impl FeatureSelection {
    pub fn validate(&self) -> Result<(), String> {
        for feature in &self.features {
            validate_feature(feature)?;
        }
        Ok(())
    }
}

fn validate_feature(feature: &str) -> Result<(), String> {
    if feature.is_empty() || feature.contains(',') || feature.chars().any(char::is_whitespace) {
        return Err(format!("feature must name one nonempty feature: {feature}"));
    }
    Ok(())
}
