// Validate literal filesystem inputs and generated Rust function paths.
// User text must not become executable connection code through interpolation.
use std::path::Path;

pub fn executable_artifact(path: &Path) -> Result<(), String> {
    let metadata = std::fs::symlink_metadata(path)
        .map_err(|error| format!("artifact {}: {error}", path.display()))?;
    if !metadata.is_file() || metadata.len() == 0 {
        return Err(format!(
            "artifact must be a nonempty regular file: {}",
            path.display()
        ));
    }
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        if metadata.permissions().mode() & 0o111 == 0 {
            return Err(format!("artifact must be executable: {}", path.display()));
        }
    }
    Ok(())
}

pub fn absolute_file(path: &Path) -> Result<(), String> {
    if !path.is_absolute() || !path.is_file() {
        return Err(format!("absolute file required: {}", path.display()));
    }
    Ok(())
}

pub fn identifier(value: &str) -> bool {
    let mut characters = value.chars();
    let first = characters
        .next()
        .is_some_and(|character| character == '_' || character.is_ascii_alphabetic());
    first && characters.all(|character| character == '_' || character.is_ascii_alphanumeric())
}
