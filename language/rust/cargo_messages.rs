// Purpose: reads Cargo's own build messages: each compiled file with the
//          features and settings it was built with and its debug-info bundle,
//          and the native (C) code each build script linked in.
// Never:   fills in a fact Cargo did not report: a missing field blocks.
use crate::blocked::Blocked;
use crate::build_facts::{Compiled, NativeCode};
use crate::cargo_build::Built;
use serde_json::Value;
use std::path::PathBuf;

/// Rust's metadata-only files: the compiler uses them while compiling, and
/// they hold no machine code, so they are not part of the build record.
const METADATA: &str = ".rmeta";

/// Apple's debug-info bundle for a program is the program's path plus this.
const DEBUG_BUNDLE: &str = ".dSYM";

/// The compiled files of one artifact message, each with its debug-info
/// bundle when Cargo reports one.
pub fn files_of(message: &Value) -> Result<Vec<Built>, Blocked> {
    let kind = message["target"]["kind"][0].as_str().unwrap_or("unknown");
    let features = names(&message["features"]);
    let files = names(&message["filenames"]);
    let bundles: Vec<&String> = files.iter().filter(|f| f.ends_with(DEBUG_BUNDLE)).collect();
    let mut found = Vec::new();
    for path in files
        .iter()
        .filter(|f| !f.ends_with(METADATA) && !f.ends_with(DEBUG_BUNDLE))
    {
        let bundle = format!("{path}{DEBUG_BUNDLE}");
        found.push(Built {
            kind: kind.to_string(),
            path: PathBuf::from(path),
            features: features.clone(),
            compiled: compiled(&message["profile"])?,
            debug_info: bundles.iter().find(|b| ***b == bundle).map(PathBuf::from),
        });
    }
    Ok(found)
}

/// The native libraries one build script asked to link, if it asked for any.
pub fn native_code_of(message: &Value) -> Option<NativeCode> {
    let linked_libs = names(&message["linked_libs"]);
    let package = message["package_id"].as_str()?.to_string();
    (!linked_libs.is_empty()).then_some(NativeCode {
        package,
        linked_libs,
    })
}

/// Cargo's `profile` object: the settings this file was compiled with.
fn compiled(profile: &Value) -> Result<Compiled, Blocked> {
    let missing = |field: &str| Blocked::Unreadable {
        what: "cargo build output".to_string(),
        cause: format!("a compiled file has no profile.{field}"),
    };
    let flag = |field: &str| profile[field].as_bool().ok_or_else(|| missing(field));
    Ok(Compiled {
        opt_level: text(&profile["opt_level"]).ok_or_else(|| missing("opt_level"))?,
        debuginfo: text(&profile["debuginfo"]).ok_or_else(|| missing("debuginfo"))?,
        debug_assertions: flag("debug_assertions")?,
        overflow_checks: flag("overflow_checks")?,
        test: flag("test")?,
    })
}

/// A string or number field as text; Cargo writes some settings as either.
fn text(value: &Value) -> Option<String> {
    match value {
        Value::String(text) => Some(text.clone()),
        Value::Number(number) => Some(number.to_string()),
        _ => None,
    }
}

fn names(list: &Value) -> Vec<String> {
    let items = list.as_array().cloned().unwrap_or_default();
    items
        .iter()
        .filter_map(Value::as_str)
        .map(str::to_string)
        .collect()
}
