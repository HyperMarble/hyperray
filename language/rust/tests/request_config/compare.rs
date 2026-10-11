// Compares an adapter build with Cargo using the same native configuration.
// Never derives expected wrapper use from the adapter's own record.
use language_rust::record::BuildRecord;
use std::ffi::OsString;
use std::path::Path;
use std::process::Command;
use std::sync::{
    Mutex,
    MutexGuard,
    PoisonError, //
};

static ONE_AT_A_TIME: Mutex<()> = Mutex::new(());

/// Held by each test for its whole run. Two of them change the process
/// environment, and every build started meanwhile would inherit it.
pub fn alone() -> MutexGuard<'static, ()> {
    ONE_AT_A_TIME.lock().unwrap_or_else(PoisonError::into_inner)
}

pub fn build(root: &Path, arguments: Vec<OsString>) -> Result<Box<BuildRecord>, String> {
    let expected = control(root, &arguments)?;
    let record = super::record::build(root, arguments)?;
    let actual = log(root)?;
    assert_eq!(actual.is_empty(), expected.is_empty());
    for line in expected.lines() {
        assert!(actual.lines().any(|observed| observed == line));
    }
    Ok(record)
}

fn control(root: &Path, arguments: &[OsString]) -> Result<String, String> {
    let control = Command::new("cargo")
        .args(["build", "--locked", "--offline", "--message-format=json"])
        .args(arguments)
        .arg("--target-dir")
        .arg(root.join("control"))
        .current_dir(root)
        .output()
        .map_err(|error| error.to_string())?;
    if !control.status.success() {
        return Err(String::from_utf8_lossy(&control.stderr).into_owned());
    }
    let expected = log(root)?;
    if root.join("wrapper.log").exists() {
        std::fs::remove_file(root.join("wrapper.log")).map_err(|error| error.to_string())?;
    }
    Ok(expected)
}

fn log(root: &Path) -> Result<String, String> {
    match std::fs::read_to_string(root.join("wrapper.log")) {
        Ok(text) => Ok(text),
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(String::new()),
        Err(error) => Err(error.to_string()),
    }
}
