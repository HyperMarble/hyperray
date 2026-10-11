// Checks an owner configuration through direct Cargo and the adapter.
// Never substitutes adapter output for the expected behavior.
use super::{
    compare,
    project, //
};

#[test]
fn inline_and_joined_wrapper_overrides_reach_cargo_and_the_record() -> Result<(), String> {
    let _alone = compare::alone();
    let root = project::create("request_inline_wrappers")?;
    let arguments = vec![
        "--config".into(),
        "build.rustc-wrapper=\"./requested-outer.sh\"".into(),
        "--config=build.rustc-workspace-wrapper=\"./requested-workspace.sh\"".into(),
    ];
    compare::build(&root, arguments)?;
    Ok(())
}
