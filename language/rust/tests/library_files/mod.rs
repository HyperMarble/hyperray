// Purpose: reads the functions of a crate out of the library files a record
//          names, with `nm`, for tests that compare one file with another.
// Never:   decides by hand which functions a library holds.
use crate::sample_project::{
    create,
    Sample, //
};
use language_rust::build_record;
use serde_json::Value;
use std::collections::BTreeSet;
use std::process::Command;

/// The record of a library crate called `name` holding `source`.
pub fn record_of(name: &str, source: &str) -> Result<Value, String> {
    let root = create(&Sample {
        name,
        file: "lib.rs",
        source,
        locked: true,
    })?;
    serde_json::to_value(build_record(&root)).map_err(|error| error.to_string())
}

/// The path of the first file in the record whose first kind is `kind`.
pub fn file_of_kind(record: &Value, kind: &str) -> Option<String> {
    let files = record["artifacts"].as_array()?;
    let found = files.iter().find(|file| file["kind"][0] == kind)?;
    found["file"]["path"].as_str().map(str::to_string)
}

/// The crate's own functions in `file`, without the part of each name that
/// changes from one build of the crate to the next.
pub fn functions(file: &str, name: &str) -> Result<BTreeSet<String>, String> {
    let listed = Command::new("nm").arg(file).output();
    let listed = listed.map_err(|error| error.to_string())?;
    let text = String::from_utf8_lossy(&listed.stdout).into_owned();
    let ours = format!("{}{name}", name.len());
    let code = text
        .lines()
        .filter(|line| line.contains(" T ") || line.contains(" t "));
    let names = code.filter_map(|line| line.split_whitespace().last());
    let kept = names.filter_map(|symbol| symbol.split_once(&ours));
    Ok(kept.map(|(_, rest)| rest.to_string()).collect())
}
