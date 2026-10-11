// Purpose: a library is also handed over in its finished form, the one Cargo
//          links, and every function in its `.rlib` is in that finished file.
// Never:   decides by hand which functions a library holds: `nm` reads both.
mod sample_project;

use language_rust::build_record;
use sample_project::{
    create,
    Sample, //
};
use serde_json::Value;
use std::collections::BTreeSet;
use std::process::Command;

const LIBRARY: &str =
    "pub fn total(each: u64, count: u64) -> u64 {\n    helper(each) * count\n}\n\n\
fn helper(x: u64) -> u64 {\n    x + 1\n}\n\npub struct Shelf {\n    pub left: u64,\n}\n\n\
impl Shelf {\n    pub fn low(&self) -> bool {\n        self.left < 5\n    }\n}\n";

fn record_of(name: &str, source: &str) -> Result<Value, String> {
    let root = create(&Sample {
        name,
        file: "lib.rs",
        source,
        locked: true,
    })?;
    serde_json::to_value(build_record(&root)).map_err(|error| error.to_string())
}

fn file_of_kind(record: &Value, kind: &str) -> Option<String> {
    let files = record["artifacts"].as_array()?;
    let found = files.iter().find(|file| file["kind"][0] == kind)?;
    found["file"]["path"].as_str().map(str::to_string)
}

/// The crate's own functions in `file`, without the part of each name that
/// changes from one build of the crate to the next.
fn functions(file: &str, name: &str) -> Result<BTreeSet<String>, String> {
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

#[test]
fn every_function_in_a_library_is_in_its_finished_form() -> Result<(), String> {
    let record = record_of("finished_whole", LIBRARY)?;
    let library = file_of_kind(&record, "lib").ok_or("the library file is not in the record")?;
    let finished =
        file_of_kind(&record, "finished library").ok_or("no finished library in the record")?;
    let wanted = functions(&library, "finished_whole")?;
    let found = functions(&finished, "finished_whole")?;
    assert!(wanted.len() >= 3, "nm found {wanted:?} in {library}");
    assert!(
        wanted.is_subset(&found),
        "missing from the finished library: {:?}",
        wanted.difference(&found)
    );
    Ok(())
}

#[test]
fn a_library_cargo_cannot_finish_says_so_in_cargos_words() -> Result<(), String> {
    let record = record_of(
        "finished_refused",
        "#![no_std]\n\npub fn one() -> u8 {\n    1\n}\n",
    )?;
    assert_eq!(record["status"], "built");
    let reason = record["not_built"][0]["reason"]
        .as_str()
        .unwrap_or_default();
    assert!(
        reason.contains("panic_handler"),
        "not built: {}",
        record["not_built"]
    );
    Ok(())
}
