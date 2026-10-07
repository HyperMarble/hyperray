// Purpose: a project's own `.cargo/config.toml` changes the build and is
//          named in the record; a project without one lists none of its own.
// Never:   depends on the settings files of this machine's home folder.
mod sample_project;

use language_rust::build_record;
use language_rust::record::{BuildRecord, Outcome};
use sample_project::{create, Sample};
use sha2::{Digest, Sha256};

const SETTINGS: &str = "[profile.release]\noverflow-checks = true\n";

fn built(name: &str, settings: Option<&str>) -> Result<(BuildRecord, String), String> {
    let root = create(&Sample {
        name,
        file: "lib.rs",
        source: "pub fn total(a: u8, b: u8) -> u8 { a + b }\n",
        locked: true,
    })?;
    if let Some(text) = settings {
        std::fs::create_dir_all(root.join(".cargo")).map_err(|error| error.to_string())?;
        std::fs::write(root.join(".cargo/config.toml"), text).map_err(|error| error.to_string())?;
    }
    let root = root.canonicalize().map_err(|error| error.to_string())?;
    match build_record(&root) {
        Outcome::Built(record) => Ok((*record, root.display().to_string())),
        Outcome::Blocked { reason } => Err(reason),
    }
}

#[test]
fn the_projects_settings_file_is_applied_and_recorded() -> Result<(), String> {
    let (record, root) = built("with_settings_file", Some(SETTINGS))?;
    let file = &record.settings.config_files[0];
    assert_eq!(file.path, format!("{root}/.cargo/config.toml"));
    assert_eq!(file.sha256, format!("{:x}", Sha256::digest(SETTINGS)));
    assert!(record.artifacts[0].compiled.overflow_checks);
    Ok(())
}

#[test]
fn a_project_without_a_settings_file_lists_none_of_its_own() -> Result<(), String> {
    let (record, root) = built("without_settings_file", None)?;
    let own: Vec<_> = record
        .settings
        .config_files
        .iter()
        .filter(|file| file.path.starts_with(&root))
        .collect();
    assert!(own.is_empty(), "{own:?}");
    assert!(!record.artifacts[0].compiled.overflow_checks);
    Ok(())
}
