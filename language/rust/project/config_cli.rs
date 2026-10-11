// Reads the configuration values named by the actual Cargo request.
// Never rewrites native arguments or invents a configuration filename.
use crate::blocked::Blocked;
use std::path::Path;

/// The `--config` value as an inline table, or nothing when it names a file
/// that exists, which Cargo then loads as a config file. A value that cannot
/// be a file name at all is not a file, as Cargo treats it.
pub(crate) fn inline_table(root: &Path, value: &str) -> Result<Option<toml::Table>, Blocked> {
    let file = root.join(value);
    let exists = file.try_exists().unwrap_or(false);
    if !value.is_empty() && exists {
        return Ok(None);
    }
    toml::from_str(value).map(Some).map_err(invalid)
}

fn invalid(cause: impl std::fmt::Display) -> Blocked {
    Blocked::Unreadable {
        what: "Cargo --config".to_string(),
        cause: cause.to_string(),
    }
}
