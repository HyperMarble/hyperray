// Purpose: Cargo selects the requested features and the project's own profile.
// Never: labels a file with features or settings that Cargo did not report.
#[path = "support/cargo_arguments.rs"]
mod cargo_arguments;
#[path = "features/project.rs"]
mod project;
mod sample_project;

use language_rust::cargo_request::Arguments;
use language_rust::record::Artifact;

/// True when Cargo compiled the file with overflow checks on.
fn has_overflow_checks(file: &Artifact) -> bool {
    let compiled = file.compiled.as_ref();
    compiled.is_some_and(|settings| settings.overflow_checks)
}

#[test]
fn a_feature_switched_off_is_left_out_and_switched_on_is_built() -> Result<(), String> {
    let off = project::build("features_off", Arguments::default())?;
    let arguments = cargo_arguments::for_commands(
        &["--release", "--features", "fancy"],
        &["--features", "fancy"],
    );
    let on = project::build("features_on", arguments)?;
    assert!(!project::contains_fancy_function(&off)?);
    assert!(project::contains_fancy_function(&on)?);
    assert!(off
        .artifacts
        .iter()
        .all(|artifact| { artifact.features.is_empty() }));
    assert!(on
        .artifacts
        .iter()
        .any(|artifact| { artifact.features == ["fancy"] }));
    Ok(())
}

#[test]
fn the_projects_own_profile_is_used_and_recorded() -> Result<(), String> {
    let arguments = cargo_arguments::for_commands(&["--release"], &[]);
    let release = project::build("profile_release", arguments)?;
    let arguments =
        cargo_arguments::for_commands(&["--profile", "dist", "--bin", "profile_dist"], &[]);
    let dist = project::build("profile_dist", arguments)?;
    assert!(release
        .artifacts
        .iter()
        .all(|artifact| { !has_overflow_checks(artifact) }));
    assert!(dist
        .artifacts
        .iter()
        .all(|artifact| { has_overflow_checks(artifact) }));
    let request = dist.cargo_request.as_ref().ok_or("no Cargo request")?;
    let arguments: Vec<_> = request
        .arguments
        .iter()
        .filter_map(|argument| argument.text.as_deref())
        .collect();
    assert!(arguments
        .windows(2)
        .any(|pair| pair == ["--profile", "dist"]));
    assert!(arguments
        .windows(2)
        .any(|pair| pair == ["--bin", "profile_dist"]));
    Ok(())
}
