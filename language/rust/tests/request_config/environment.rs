// Checks Cargo's dedicated and generic tool-variable precedence.
// Never leaves a changed process environment for another test.
use super::{
    compare,
    project, //
};
use std::ffi::OsString;

struct Restore {
    name: &'static str,
    previous: Option<OsString>,
}

impl Drop for Restore {
    fn drop(&mut self) {
        match &self.previous {
            Some(value) => std::env::set_var(self.name, value),
            None => std::env::remove_var(self.name),
        }
    }
}

fn selected(name: &'static str, expected: &str, project_name: &str) -> Result<(), String> {
    let _alone = compare::alone();
    let root = project::create(project_name)?;
    let _restore = Restore {
        name,
        previous: std::env::var_os(name),
    };
    std::env::set_var(name, root.join("outer-wrapper.sh"));
    let arguments = vec![
        "--config".into(),
        "build.rustc-wrapper=\"./requested-outer.sh\"".into(),
    ];
    let record = compare::build(&root, arguments)?;
    let variable = record
        .environment
        .iter()
        .find(|variable| variable.name.as_deref() == Some(name))
        .ok_or("inherited wrapper environment missing")?;
    assert!(variable.value.is_none());
    assert!(!variable.value_sha256.is_empty());
    assert!(root.join(expected).is_file());
    Ok(())
}

#[test]
fn cli_configuration_overrides_the_generic_tool_variable() -> Result<(), String> {
    selected(
        "CARGO_BUILD_RUSTC_WRAPPER",
        "requested-outer.sh",
        "request_generic_environment",
    )
}

#[test]
fn the_dedicated_tool_variable_overrides_cli_configuration() -> Result<(), String> {
    selected(
        "RUSTC_WRAPPER",
        "outer-wrapper.sh",
        "request_dedicated_environment",
    )
}
