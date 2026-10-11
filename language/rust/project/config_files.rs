// Purpose: records Cargo config files and optional includes for a build.
// Never:   hashes an included file without parsing its Cargo include rules.
#[path = "config_cli.rs"]
mod cli;
#[path = "config_cli_arguments.rs"]
mod cli_arguments;
#[path = "config_includes.rs"]
mod includes;

use crate::blocked::Blocked;
use crate::record::FileDigest;
use std::ffi::OsString;
use std::path::{
    Path,
    PathBuf, //
};

/// Config files Cargo loads and optional includes it could not find.
#[derive(Debug, PartialEq)]
pub struct ConfigReport {
    pub files: Vec<FileDigest>,
    pub missing_optional: Vec<String>,
}

/// Every config Cargo reads for a build of `root` with `arguments`: each
/// folder's file from `root` up, the home one, then every `--config` the
/// request names, with the files each of them includes.
pub fn report_for_request(root: &Path, arguments: &[OsString]) -> Result<ConfigReport, Blocked> {
    let root = root
        .canonicalize()
        .map_err(|error| unreadable(root, error))?;
    let mut collector = includes::Collector::default();
    for folder in config_folders(&root) {
        if let Some(file) = config_file(&folder) {
            collector.load(&file)?;
        }
    }
    for argument in cli_arguments::values(arguments)? {
        match cli::inline_table(&root, &argument)? {
            Some(table) => collector.load_inline(&root, &table)?,
            None => collector.load(&root.join(argument))?,
        }
    }
    Ok(collector.finish())
}

fn config_folders(root: &Path) -> impl Iterator<Item = PathBuf> + '_ {
    root.ancestors()
        .map(|folder| folder.join(".cargo"))
        .chain(cargo_home())
}

fn config_file(folder: &Path) -> Option<PathBuf> {
    ["config", "config.toml"]
        .iter()
        .map(|name| folder.join(name))
        .find(|path| path.is_file())
}

fn cargo_home() -> Option<PathBuf> {
    std::env::var_os("CARGO_HOME")
        .map(PathBuf::from)
        .or_else(|| std::env::var_os("HOME").map(|home| PathBuf::from(home).join(".cargo")))
}

fn unreadable(path: &Path, error: std::io::Error) -> Blocked {
    Blocked::Unreadable {
        what: path.display().to_string(),
        cause: error.to_string(),
    }
}
