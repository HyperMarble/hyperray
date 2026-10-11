// Purpose: asks the owners which of the compiler's own libraries a record's
//          built files load at run time. Apple's `otool` lists the files
//          each one loads, and the compiler says which folder its own are in.
// Never:   decides by hand what a built file loads.
use serde_json::Value;
use std::collections::BTreeSet;
use std::path::Path;
use std::process::Command;

/// What the record calls a library the compiler ships ready made.
const KIND: &str = "compiler library";

fn printed(program: &str, words: &[&str], folder: &Path) -> Result<String, String> {
    let output = Command::new(program)
        .args(words)
        .current_dir(folder)
        .output();
    let output = output.map_err(|error| error.to_string())?;
    Ok(String::from_utf8_lossy(&output.stdout).into_owned())
}

fn files(record: &Value) -> Vec<Value> {
    record["artifacts"].as_array().cloned().unwrap_or_default()
}

fn path_of(file: &Value) -> String {
    let path = file["file"]["path"].as_str();
    path.unwrap_or_default().to_string()
}

/// The compiler's libraries the record hands over, by path.
pub fn handed_over(record: &Value) -> BTreeSet<String> {
    let mut found = BTreeSet::new();
    for file in files(record) {
        if file["kind"][0] == KIND {
            found.insert(path_of(&file));
        }
    }
    found
}

/// The files in the compiler's `folder` that `otool -L` says `file` loads.
/// Each line `otool` prints starts with the path of one loaded file.
fn loaded_from(folder: &Path, file: &str) -> Result<Vec<String>, String> {
    let listed = printed("otool", &["-L", file], folder)?;
    let mut found = Vec::new();
    for line in listed.lines() {
        let loaded = line.trim().split(" (").next();
        let Some(name) = Path::new(loaded.unwrap_or_default()).file_name() else {
            continue;
        };
        if folder.join(name).is_file() {
            found.push(folder.join(name).display().to_string());
        }
    }
    Ok(found)
}

/// The compiler's libraries that the files this build made load, by path.
pub fn owners_answer(root: &Path, record: &Value) -> Result<BTreeSet<String>, String> {
    let folder = printed("rustc", &["--print", "target-libdir"], root)?;
    let folder = Path::new(folder.trim());
    let mut wanted = BTreeSet::new();
    for file in files(record) {
        if file["kind"][0] != KIND {
            wanted.extend(loaded_from(folder, &path_of(&file))?);
        }
    }
    Ok(wanted)
}
