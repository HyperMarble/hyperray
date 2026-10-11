// Purpose: names each library of the project the build compiled, with what
//          Cargo reported about it.
// Never:   adds a library Cargo did not report.
use serde_json::Value;
use std::path::PathBuf;

/// A library the build compiled: its package, where its Cargo.toml is, the
/// crate's name in code, and the features Cargo built it with.
#[derive(Clone, Debug, PartialEq)]
pub struct Library {
    pub package: String,
    pub manifest: PathBuf,
    pub name: String,
    pub features: Vec<String>,
}

/// The library a build message reports, when it is a Rust library built
/// for use and not for its tests: the file Cargo makes for it is an `.rlib`.
pub fn library_of(message: &Value) -> Option<Library> {
    let types = strings(&message["target"]["crate_types"]);
    let rust_library = types.iter().any(|kind| kind == "lib" || kind == "rlib");
    if !rust_library || message["profile"]["test"] != false {
        return None;
    }
    Some(Library {
        package: message["package_id"].as_str()?.to_string(),
        manifest: PathBuf::from(message["manifest_path"].as_str()?),
        name: message["target"]["name"].as_str()?.replace('-', "_"),
        features: strings(&message["features"]),
    })
}

fn strings(list: &Value) -> Vec<String> {
    let items = list.as_array().cloned().unwrap_or_default();
    let texts = items.iter().filter_map(Value::as_str);
    texts.map(str::to_string).collect()
}
