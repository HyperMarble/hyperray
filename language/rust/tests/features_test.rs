// Purpose: Cargo selects the requested features and the project's own profile.
// Never: labels a file with features or settings that Cargo did not report.
#[path = "support/cargo_arguments.rs"]
mod cargo_arguments;
#[path = "features/project.rs"]
mod project;
mod sample_project;

use language_rust::cargo_request::Arguments;

#[test]
fn a_feature_switched_off_is_left_out_and_switched_on_is_built() -> Result<(), String> {
    let off = project::build("features_off", Arguments::default())?;
    let arguments = cargo_arguments::for_commands(
        &["--release", "--features", "fancy"],
        &["--features", "fancy"],
        &["--release", "--features", "fancy"],
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
    let arguments = cargo_arguments::for_commands(&["--release"], &[], &["--release"]);
    let release = project::build("profile_release", arguments)?;
    let arguments = cargo_arguments::for_commands(
        &["--profile", "dist", "--bin", "profile_dist"],
        &[],
        &["--profile", "dist"],
    );
    let dist = project::build("profile_dist", arguments)?;
    assert!(release
        .artifacts
        .iter()
        .all(|artifact| { !artifact.compiled.overflow_checks }));
    assert!(dist
        .artifacts
        .iter()
        .all(|artifact| { artifact.compiled.overflow_checks }));
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
