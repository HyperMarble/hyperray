// Purpose: Test every `.hray` format artifact through the public parser.
// Never: Name a fixed artifact count or skip an unreadable file.
// In: All `.hray` files below `contract/format`.
// Out: Acceptance for every current and future format artifact.
// Fails: A format artifact is absent, unreadable, or invalid SMT-shaped source.

use contract::parse;
use std::ffi::OsStr;
use std::fs;
use std::path::{Path, PathBuf};

fn contract_files(directory: &Path) -> Result<Vec<PathBuf>, String> {
    let entries = fs::read_dir(directory).map_err(|error| error.to_string())?;
    let mut files = Vec::new();
    for entry in entries {
        let path = entry.map_err(|error| error.to_string())?.path();
        if path.is_dir() {
            files.extend(contract_files(&path)?);
            continue;
        }
        if path.extension() == Some(OsStr::new("hray")) {
            files.push(path);
        }
    }
    Ok(files)
}

fn parse_contract_files() -> Result<(), String> {
    let root = Path::new(env!("CARGO_MANIFEST_DIR")).join("format");
    let files = contract_files(&root)?;
    if files.is_empty() {
        return Err("contract/format contains no .hray files".into());
    }
    for path in files {
        let source = fs::read_to_string(&path).map_err(|error| error.to_string())?;
        parse(&source).map_err(|error| format!("{}: {error}", path.display()))?;
    }
    Ok(())
}

#[test]
fn every_format_file_uses_the_public_grammar() {
    assert_eq!(parse_contract_files(), Ok(()));
}
