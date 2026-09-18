// Span tests keep each finding inside the compiler-named changed function.

use crate::common;
use crate::shape_clippy_location::tree_for;
use hyperray_rust::extract::{change, crate_dir, manifest, Located, Manifest};
use hyperray_rust::shape::{run, Finding};
use std::path::Path;

#[test]
fn every_finding_in_a_changed_function_sits_inside_its_span() {
    let Some(sources) = common::dir("HYPERRAY_FIXTURE_SRC") else {
        return;
    };
    let mut ran = 0;
    let mut failures: Vec<String> = Vec::new();
    for (name, text) in common::patches() {
        let Some(tree) = tree_for(&sources, &name) else {
            continue;
        };
        match measured(&tree, &text) {
            Ok(escapes) => {
                failures.extend(escapes);
                ran += 1;
            }
            Err(reason) => failures.push(format!("{name}: {reason}")),
        }
    }
    assert!(failures.is_empty(), "{}", failures.join("\n"));
    assert!(ran > 0, "no fixture had a source tree");
}

fn measured(tree: &Path, patch: &str) -> Result<Vec<String>, String> {
    let Ok(built) = manifest(tree, &change(patch)) else {
        return Err("manifest failed".to_string());
    };
    let Some(first) = built.functions.first() else {
        return Err("the patch touches no function".to_string());
    };
    let Some(dir) = crate_dir(tree, &first.path) else {
        return Err(format!("no crate holds {}", first.path));
    };
    let done = clippy_in(&dir)?;
    Ok(done
        .iter()
        .filter_map(|finding| escaped(&built, tree, &dir, finding))
        .collect())
}

fn clippy_in(dir: &Path) -> Result<Vec<Finding>, String> {
    match run(dir) {
        Err(error) => Err(format!("clippy did not run: {error}")),
        Ok(done) if done.exit_code != Some(0) => Err("clippy failed".to_string()),
        Ok(done) => Ok(done.findings),
    }
}

fn escaped(built: &Manifest, tree: &Path, dir: &Path, finding: &Finding) -> Option<String> {
    let in_tree = dir.join(&finding.path);
    let relative = in_tree.strip_prefix(tree).ok()?.display().to_string();
    let owner = built
        .functions
        .iter()
        .find(|function| owns(function, &relative, finding))?;
    match finding.line_end <= owner.end_line {
        true => None,
        false => Some(format!("{}: {} escapes its span", owner.name, finding.lint)),
    }
}

fn owns(function: &Located, path: &str, finding: &Finding) -> bool {
    let span = function.start_line..=function.end_line;
    function.path == path && span.contains(&finding.line_start)
}
