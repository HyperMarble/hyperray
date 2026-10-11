// Purpose: configures owner wrappers and an included relative environment.
// Never: names a wrapper implementation in production adapter code.
use std::os::unix::fs::PermissionsExt;
use std::path::Path;

const EXECUTABLE_PERMISSIONS: u32 = 0o755;
const OUTER_WRAPPER: &str = "#!/bin/sh\nprintf 'outer\\n' >> \"$OWNER_WRAPPER_LOG\"\nexec \"$@\"\n";
const WORKSPACE_WRAPPER: &str =
    "#!/bin/sh\nprintf 'workspace\\n' >> \"$OWNER_WRAPPER_LOG\"\nexec \"$@\"\n";
const CARGO_CONFIG: &str = r#"include = ["env.toml"]

[build]
rustc-wrapper = "./outer-wrapper.sh"
rustc-workspace-wrapper = "./workspace-wrapper.sh"
"#;

pub fn configure(root: &Path) -> Result<String, String> {
    write_wrapper(root, "outer-wrapper.sh", OUTER_WRAPPER)?;
    write_wrapper(root, "workspace-wrapper.sh", WORKSPACE_WRAPPER)?;
    write_cargo_configuration(root)?;
    write_environment_configuration(root)?;
    Ok(root.join("relative/value").display().to_string())
}

pub(super) fn write_wrapper(root: &Path, name: &str, source: &str) -> Result<(), String> {
    let path = root.join(name);
    std::fs::write(&path, source).map_err(|error| error.to_string())?;
    let mut permissions = std::fs::metadata(&path)
        .map_err(|error| error.to_string())?
        .permissions();
    permissions.set_mode(EXECUTABLE_PERMISSIONS);
    std::fs::set_permissions(path, permissions).map_err(|error| error.to_string())
}

fn write_cargo_configuration(root: &Path) -> Result<(), String> {
    let cargo_directory = root.join(".cargo");
    std::fs::create_dir(&cargo_directory).map_err(|error| error.to_string())?;
    std::fs::write(cargo_directory.join("config.toml"), CARGO_CONFIG)
        .map_err(|error| error.to_string())
}

fn write_environment_configuration(root: &Path) -> Result<(), String> {
    let wrapper_log = root.join("wrapper.log").display().to_string();
    let log_setting = format!("OWNER_WRAPPER_LOG = {{ value = {wrapper_log:?}, force = true }}");
    let relative_setting =
        "OWNER_RELATIVE = { value = \"relative/value\", relative = true, force = true }";
    let environment = ["[env]", &log_setting, relative_setting, ""].join("\n");
    std::fs::write(root.join(".cargo/env.toml"), environment).map_err(|error| error.to_string())
}
