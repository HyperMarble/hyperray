// Policy tests prove that Hyperray does not add its Clippy limits to targets.

use hyperray_rust::shape::run;
use std::path::PathBuf;

#[test]
fn a_target_without_policy_does_not_receive_hyperray_policy(
) -> Result<(), Box<dyn std::error::Error>> {
    let root = temporary_crate()?;
    let measured = run(&root);
    std::fs::remove_dir_all(&root)?;
    let done = measured?;
    assert_eq!(done.exit_code, Some(0));
    assert!(!done
        .findings
        .iter()
        .any(|finding| finding.lint == "clippy::too_many_lines"));
    Ok(())
}

fn temporary_crate() -> std::io::Result<PathBuf> {
    let root = std::env::temp_dir().join(format!("hyperray-shape-policy-{}", std::process::id()));
    if root.exists() {
        std::fs::remove_dir_all(&root)?;
    }
    std::fs::create_dir_all(root.join("src"))?;
    std::fs::write(
        root.join("Cargo.toml"),
        "[package]\nname = \"shape_policy\"\nversion = \"0.1.0\"\nedition = \"2021\"\n",
    )?;
    std::fs::write(root.join("src/lib.rs"), long_function())?;
    Ok(root)
}

fn long_function() -> String {
    let mut source = String::from("pub fn classify(value: u8) -> u8 {\n    match value {\n");
    for value in 0..42 {
        source.push_str(&format!("        {value} => {value},\n"));
    }
    source.push_str("        _ => value,\n    }\n}\n");
    source
}
