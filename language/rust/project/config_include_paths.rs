// Purpose: parses and resolves Cargo's supported config include forms.
// Never:   treats an invalid required path as an optional omission.
use crate::blocked::Blocked;
use std::path::{
    Path,
    PathBuf, //
};

pub(super) use crate::project::config_include_io::{
    cycle,
    read_config,
    unreadable, //
};

pub(super) fn include_paths(
    value: &toml::Value,
    parent: &Path,
) -> Result<Vec<(PathBuf, bool)>, Blocked> {
    let values = value
        .as_array()
        .ok_or_else(|| invalid(parent, "include must be an array"))?;
    values
        .iter()
        .map(|value| include_path(value, parent))
        .collect()
}

fn include_path(value: &toml::Value, parent: &Path) -> Result<(PathBuf, bool), Blocked> {
    if let Some(path) = value.as_str() {
        return checked_path(PathBuf::from(path), false, parent);
    }
    let table = value
        .as_table()
        .ok_or_else(|| invalid(parent, "include entry must be string or table"))?;
    let path = table
        .get("path")
        .and_then(toml::Value::as_str)
        .ok_or_else(|| invalid(parent, "include table needs a string path"))?;
    let optional = match table.get("optional") {
        None => false,
        Some(value) => value
            .as_bool()
            .ok_or_else(|| invalid(parent, "include optional must be a boolean"))?,
    };
    checked_path(PathBuf::from(path), optional, parent)
}

fn checked_path(path: PathBuf, optional: bool, parent: &Path) -> Result<(PathBuf, bool), Blocked> {
    if path.extension().and_then(|extension| extension.to_str()) != Some("toml") {
        return Err(invalid(parent, "include path must end in .toml"));
    }
    Ok((path, optional))
}

pub(super) fn resolve_include(file: &Path, include: &Path) -> PathBuf {
    file.parent().map_or(file, |parent| parent).join(include)
}

fn invalid(file: &Path, cause: &str) -> Blocked {
    Blocked::Unreadable {
        what: file.display().to_string(),
        cause: cause.to_string(),
    }
}
