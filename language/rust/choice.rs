// Purpose: the build the user asks for: which profile and which features.
// Never:   switches on a feature or picks a profile nobody asked for; with no
//          request it is exactly the project's own default release build.
use serde::Serialize;

/// Which optional parts of the project are switched on.
#[derive(Debug, PartialEq, Clone, Serialize)]
#[serde(tag = "kind", content = "names", rename_all = "lowercase")]
pub enum Features {
    /// The project's own default features.
    Default,
    /// The project's default features, plus these.
    Plus(Vec<String>),
    /// Only these: the project's default features are switched off.
    Only(Vec<String>),
    /// Every feature the project has.
    All,
}

/// One build of the project: a named profile and a feature setting.
#[derive(Debug, PartialEq, Clone, Serialize)]
pub struct Choice {
    pub profile: String,
    pub features: Features,
}

impl Default for Choice {
    /// The project's own release build, with its default features.
    fn default() -> Self {
        Choice {
            profile: "release".to_string(),
            features: Features::Default,
        }
    }
}

impl Choice {
    /// The arguments Cargo itself takes for this build.
    pub fn cargo_args(&self) -> Vec<String> {
        let mut args = vec!["--profile".to_string(), self.profile.clone()];
        match &self.features {
            Features::Default => {}
            Features::Plus(names) => args.extend(feature_list(names)),
            Features::Only(names) => {
                args.push("--no-default-features".to_string());
                args.extend(feature_list(names));
            }
            Features::All => args.push("--all-features".to_string()),
        }
        args
    }
}

/// `--features a,b`, or nothing when no feature is named.
fn feature_list(names: &[String]) -> Vec<String> {
    if names.is_empty() {
        return Vec::new();
    }
    vec!["--features".to_string(), names.join(",")]
}
