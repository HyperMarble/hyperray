// Purpose: a feature switched off leaves its code out of the built file and
//          switched on puts it in, and a project's own profile is used; the
//          record says which, as Cargo reports it.
// Never:   labels a built file with features or settings it was not built with.
mod sample_project;

use language_rust::build_record_with;
use language_rust::choice::{Choice, Features, Targets};
use language_rust::record::{BuildRecord, Outcome};
use sample_project::{create, Sample};
use std::io::Write;

const SOURCE: &str = "#[cfg(feature = \"fancy\")]\n#[inline(never)]\n\
fn fancy_total(n: u64) -> u64 { n * 3 + 1 }\n\
fn main() {\n    #[cfg(feature = \"fancy\")]\n    println!(\"{}\", fancy_total(std::hint::black_box(5)));\n}\n";

const EXTRA: &str =
    "\n[features]\nfancy = []\n\n[profile.dist]\ninherits = \"release\"\noverflow-checks = true\n";

fn built(name: &str, choice: Choice) -> Result<Box<BuildRecord>, String> {
    let root = create(&Sample {
        name,
        file: "main.rs",
        source: SOURCE,
        locked: true,
    })?;
    let mut manifest = std::fs::OpenOptions::new()
        .append(true)
        .open(root.join("Cargo.toml"))
        .map_err(|e| e.to_string())?;
    manifest
        .write_all(EXTRA.as_bytes())
        .map_err(|e| e.to_string())?;
    match build_record_with(&root, &choice) {
        Outcome::Built(record) => Ok(record),
        Outcome::Blocked { reason } => Err(reason),
    }
}

/// Whether the program's own file names `fancy_total`, the function behind the feature.
fn has_fancy(record: &BuildRecord) -> Result<bool, String> {
    let program = record
        .artifacts
        .iter()
        .find(|a| a.kind == "bin")
        .ok_or("no program")?;
    let bytes = std::fs::read(&program.file.path).map_err(|e| e.to_string())?;
    Ok(bytes.windows(11).any(|w| w == b"fancy_total"))
}

#[test]
fn a_feature_switched_off_is_left_out_and_switched_on_is_built() -> Result<(), String> {
    let off = built("features_off", Choice::default())?;
    let on = built(
        "features_on",
        Choice {
            profile: "release".to_string(),
            features: Features::Plus(vec!["fancy".to_string()]),
            targets: Targets::Default,
        },
    )?;
    assert!(!has_fancy(&off)? && has_fancy(&on)?);
    assert!(off.artifacts.iter().all(|a| a.features.is_empty()));
    assert!(on.artifacts.iter().any(|a| a.features == ["fancy"]));
    Ok(())
}

#[test]
fn the_projects_own_profile_is_used_and_recorded() -> Result<(), String> {
    let release = built("profile_release", Choice::default())?;
    let dist = built(
        "profile_dist",
        Choice {
            profile: "dist".to_string(),
            features: Features::Default,
            targets: Targets::Default,
        },
    )?;
    assert!(release
        .artifacts
        .iter()
        .all(|a| !a.compiled.overflow_checks));
    assert!(dist.artifacts.iter().all(|a| a.compiled.overflow_checks));
    assert_eq!(dist.settings.requested.profile, "dist");
    Ok(())
}
