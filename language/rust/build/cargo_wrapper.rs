// Purpose: finishes a library Cargo will not link by itself, through a tiny
//          crate of the adapter's own that only uses it. Cargo builds that
//          crate as a linked library, with the owner's library inside it.
// Never:   changes the owner's library, or stands in when the project's own
//          Cargo.toml holds build settings a separate crate would not get.
use crate::blocked::Blocked;
use crate::cargo_build::Built;
use crate::cargo_library::Library;
use crate::cargo_messages::files_of;
use crate::cargo_observation::Observation;
use crate::cargo_request::Arguments;
use crate::project::Project;
use std::ffi::OsString;
use std::path::Path;

const NOT_ADDED: &str = "not added yet: the project's Cargo.toml sets profile, patch or \
replace, and a crate of the adapter's own would be built without them\n";

/// The library's finished files through the wrapper, or the words for why
/// there are none: Cargo's own, or that the wrapper does not fit here.
pub(super) fn finish(
    project: &Project,
    supplied: &Arguments,
    library: &Library,
    folder: &Path,
) -> Result<Result<Vec<Built>, String>, Blocked> {
    if !super::cargo_wrapper_files::fits(project)? {
        return Ok(Err(NOT_ADDED.to_string()));
    }
    let manifest = super::cargo_wrapper_files::write(project, library, folder)?;
    let arguments = arguments(supplied, &manifest, folder);
    let process = crate::cargo_observation::capture(Path::new("cargo"), &arguments, &project.root)?;
    if !process.success {
        return Ok(Err(super::cargo_refusal::refusal(&process)));
    }
    files(&process, &manifest).map(Ok)
}

/// `cargo build` of the wrapper under `folder`, with the owner's arguments
/// except the picking ones. The wrapper's lock file gains the wrapper
/// itself, so `--locked` is left out and `--frozen` keeps only its offline
/// half; every other pinned version is the owner's.
fn arguments(supplied: &Arguments, manifest: &Path, folder: &Path) -> Vec<OsString> {
    let mut arguments = vec![OsString::from("build"), OsString::from("--manifest-path")];
    arguments.push(manifest.as_os_str().to_owned());
    let owners = crate::cargo_request::without_message_format(&supplied.build);
    for word in crate::cargo_request::without_picks(&owners) {
        match word.to_str() {
            Some("--locked") => {}
            Some("--frozen") => arguments.push(OsString::from("--offline")),
            _ => arguments.push(word),
        }
    }
    arguments.push(OsString::from("--target-dir"));
    arguments.push(folder.as_os_str().to_owned());
    arguments.push(OsString::from("--message-format=json"));
    arguments
}

/// The files Cargo reports for the wrapper itself, each marked finished.
fn files(process: &Observation, manifest: &Path) -> Result<Vec<Built>, Blocked> {
    let printed = String::from_utf8_lossy(&process.stdout);
    let wrapper = manifest.to_string_lossy();
    let mut found = Vec::new();
    for line in printed.lines().filter(|line| line.starts_with('{')) {
        let message = super::cargo_build_result::parse(line, "cargo build output")?;
        if message["reason"] == "compiler-artifact" && message["manifest_path"] == *wrapper {
            found.extend(files_of(&message)?);
        }
    }
    found
        .iter_mut()
        .for_each(|file| file.kind = vec![super::cargo_finished::KIND.to_string()]);
    Ok(found)
}
