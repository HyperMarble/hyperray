// Purpose: writes the adapter's own tiny crate that uses one library of the
//          project: its Cargo.toml, its one line of code, and a copy of the
//          owner's lock file so every pinned version stays the owner's.
// Never:   writes into the project's own folders.
use crate::blocked::Blocked;
use crate::cargo_library::Library;
use crate::project::Project;
use std::ffi::OsString;
use std::path::{
    Path,
    PathBuf, //
};

/// True when a separate crate gets the same build settings as the project:
/// its top Cargo.toml sets no profile, patch or replace.
pub(super) fn fits(project: &Project) -> Result<bool, Blocked> {
    let manifest = project.lock_file.with_file_name("Cargo.toml");
    let text = std::fs::read_to_string(&manifest).map_err(|error| unreadable(&manifest, error))?;
    let table: toml::Table = toml::from_str(&text).map_err(|error| unreadable(&manifest, error))?;
    let own_settings = ["profile", "patch", "replace"];
    Ok(!own_settings.iter().any(|key| table.contains_key(*key)))
}

/// Writes the wrapper for `library` under `folder` and returns its Cargo.toml.
pub(super) fn write(
    project: &Project,
    library: &Library,
    folder: &Path,
) -> Result<PathBuf, Blocked> {
    let home = folder.join("wrapper").join(&library.name);
    let saved = |file: &Path, text: String| {
        std::fs::write(file, text).map_err(|error| unreadable(file, error))
    };
    std::fs::create_dir_all(&home).map_err(|error| unreadable(&home, error))?;
    let source = library.manifest.parent().unwrap_or(&project.root);
    let quoted = |text: &str| toml::Value::from(text).to_string();
    let features = toml::Value::from(library.features.clone());
    let manifest = format!(
        "[package]\nname = \"hyperray-finished\"\nversion = \"0.0.0\"\nedition = \"2021\"\n\n\
[lib]\ncrate-type = [\"dylib\"]\npath = \"lib.rs\"\n\n[dependencies.{}]\npath = {}\n\
default-features = false\nfeatures = {features}\n\n[workspace]\n",
        package_name(project, library)?,
        quoted(&source.to_string_lossy()),
    );
    saved(&home.join("Cargo.toml"), manifest)?;
    saved(
        &home.join("lib.rs"),
        format!("#![allow(unused_imports)]\npub use {}::*;\n", library.name),
    )?;
    let lock = home.join("Cargo.lock");
    std::fs::copy(&project.lock_file, &lock).map_err(|error| unreadable(&lock, error))?;
    Ok(home.join("Cargo.toml"))
}

/// The package's name, as Cargo reads it from the library's own Cargo.toml.
fn package_name(project: &Project, library: &Library) -> Result<String, Blocked> {
    let words = [
        "metadata",
        "--no-deps",
        "--format-version",
        "1",
        "--manifest-path",
    ];
    let mut arguments: Vec<OsString> = words.into_iter().map(OsString::from).collect();
    arguments.push(library.manifest.as_os_str().to_owned());
    let process = crate::cargo_observation::capture(Path::new("cargo"), &arguments, &project.root)?;
    let text = super::cargo_build_result::stdout(&process)?;
    let metadata = super::cargo_build_result::parse(&text, "cargo metadata")?;
    let wanted = library.manifest.to_string_lossy();
    let packages = metadata["packages"].as_array().cloned().unwrap_or_default();
    let found = packages
        .iter()
        .find(|package| package["manifest_path"] == *wanted);
    let name = found.and_then(|package| package["name"].as_str());
    name.map(str::to_string).ok_or_else(|| Blocked::Unreadable {
        what: "cargo metadata".to_string(),
        cause: format!("no package for {wanted}"),
    })
}

fn unreadable(path: &Path, error: impl std::fmt::Display) -> Blocked {
    Blocked::Unreadable {
        what: path.display().to_string(),
        cause: error.to_string(),
    }
}
