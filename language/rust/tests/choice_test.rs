// Purpose: each build choice becomes exactly Cargo's own arguments.
// Never:   switches a feature on, or picks a profile, that was not asked for.
use language_rust::choice::{Choice, Features};

fn args(profile: &str, features: Features) -> Vec<String> {
    let choice = Choice {
        profile: profile.to_string(),
        features,
    };
    choice.cargo_args()
}

#[test]
fn the_default_is_the_projects_own_release_build() {
    assert_eq!(Choice::default().cargo_args(), ["--profile", "release"]);
}

#[test]
fn each_feature_setting_becomes_cargos_arguments() {
    let fancy = vec!["fancy".to_string(), "fast".to_string()];
    assert_eq!(
        args("dist", Features::Plus(fancy.clone())),
        ["--profile", "dist", "--features", "fancy,fast"]
    );
    assert_eq!(
        args("release", Features::Only(fancy)),
        [
            "--profile",
            "release",
            "--no-default-features",
            "--features",
            "fancy,fast"
        ]
    );
    assert_eq!(
        args("release", Features::Only(Vec::new())),
        ["--profile", "release", "--no-default-features"]
    );
    assert_eq!(
        args("release", Features::All),
        ["--profile", "release", "--all-features"]
    );
}
