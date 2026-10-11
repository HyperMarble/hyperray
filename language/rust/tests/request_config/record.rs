// Builds with the same native request used by the direct Cargo control.
// Never translates the caller's configuration arguments.
use language_rust::cargo_request::Arguments;
use language_rust::record::{
    BuildRecord,
    Outcome, //
};
use std::ffi::OsString;
use std::path::Path;

pub fn build(root: &Path, arguments: Vec<OsString>) -> Result<Box<BuildRecord>, String> {
    let configuration = arguments.clone();
    let mut build = arguments;
    build.extend([
        OsString::from("--offline"),
        OsString::from("--target-dir"),
        root.join("observed").into_os_string(),
    ]);
    let request = Arguments {
        build,
        metadata: configuration,
    };
    let outcome = language_rust::build_record_with(root, &request);
    let Outcome::Built(record) = outcome else {
        return Err(format!("{outcome:#?}"));
    };
    Ok(record)
}
