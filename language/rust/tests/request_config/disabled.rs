// Checks an owner configuration through direct Cargo and the adapter.
// Never substitutes adapter output for the expected behavior.
use super::{
    compare,
    project, //
};
use std::ffi::OsString;

#[test]
fn a_request_can_disable_unavailable_inherited_wrappers() -> Result<(), String> {
    let _alone = compare::alone();
    let root = project::create("request_disabled_wrappers")?;
    project::unavailable_defaults(&root)?;
    let arguments: Vec<OsString> = [
        "--config",
        "build.rustc-wrapper=\"\"",
        "--config",
        "build.rustc-workspace-wrapper=\"\"",
    ]
    .map(OsString::from)
    .into();
    compare::build(&root, arguments)?;
    Ok(())
}
