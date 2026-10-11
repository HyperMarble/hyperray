// Purpose: reads Cargo include files and names their filesystem errors.
// Never: treats an unreadable or cyclic required include as optional.
use crate::blocked::Blocked;
use std::path::Path;

pub(super) fn read_config(file: &Path) -> Result<toml::Table, Blocked> {
    let text = std::fs::read_to_string(file).map_err(|error| unreadable(file, error))?;
    toml::from_str(&text).map_err(|error: toml::de::Error| Blocked::Unreadable {
        what: file.display().to_string(),
        cause: error.to_string(),
    })
}

pub(super) fn unreadable(path: &Path, error: std::io::Error) -> Blocked {
    Blocked::Unreadable {
        what: path.display().to_string(),
        cause: error.to_string(),
    }
}

pub(super) fn cycle(file: &Path) -> Blocked {
    Blocked::Unreadable {
        what: file.display().to_string(),
        cause: "Cargo config include cycle".to_string(),
    }
}
