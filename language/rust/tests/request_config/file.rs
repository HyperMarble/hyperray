// Checks an owner configuration through direct Cargo and the adapter.
// Never substitutes adapter output for the expected behavior.
use super::{
    compare,
    project, //
};

#[test]
fn a_request_file_and_its_include_are_applied_and_hashed() -> Result<(), String> {
    let _alone = compare::alone();
    let root = project::create("request_file_wrappers")?;
    project::file_configuration(&root)?;
    let record = compare::build(&root, vec!["--config".into(), ".cargo/request.toml".into()])?;
    for name in ["request.toml", "request-values.toml"] {
        let path = root.join(".cargo").join(name);
        assert!(record
            .settings
            .config_files
            .iter()
            .any(|file| file.path == path.display().to_string()));
    }
    Ok(())
}
