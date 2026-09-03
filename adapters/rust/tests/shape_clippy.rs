// The rules stage 2 must keep: a finding names a shape lint with a real
// place, and a finding inside a changed function stays within that
// function's span. No fixture name is known here.

mod common;

use hyperray_rust::extract::{change, crate_dir, manifest, Located, Manifest};
use hyperray_rust::shape::{findings_in, run, Finding};
use std::path::{Path, PathBuf};

// Every `*.jsonl` under HYPERRAY_FIXTURES is a `cargo clippy
// --message-format=json` run captured verbatim with the shape lints on.
#[test]
fn every_clippy_capture_yields_only_shape_lints_with_a_primary_span() {
    let Some(root) = common::dir("HYPERRAY_FIXTURES") else {
        return;
    };
    let failures: Vec<String> = captures_in(&root)
        .iter()
        .flat_map(|log| findings_in(log))
        .filter_map(named)
        .collect();
    assert!(failures.is_empty(), "{}", failures.join("\n"));
}

fn captures_in(root: &Path) -> Vec<String> {
    let Ok(entries) = std::fs::read_dir(root) else {
        return Vec::new();
    };
    entries
        .filter_map(|entry| entry.ok().map(|found| found.path()))
        .filter(|path| path.extension().is_some_and(|kind| kind == "jsonl"))
        .filter_map(|path| std::fs::read_to_string(path).ok())
        .collect()
}

fn named(finding: Finding) -> Option<String> {
    let lint = finding.lint.starts_with("clippy::");
    let file = finding.path.ends_with(".rs");
    let lines = finding.line_start >= 1 && finding.line_start <= finding.line_end;
    let good = lint && file && lines && !finding.message.is_empty();
    match good {
        true => None,
        false => Some(format!("{} at {}", finding.lint, finding.path)),
    }
}

#[test]
fn a_non_json_line_is_skipped() {
    assert!(findings_in("Compiling foo\nwarning: bar\n").is_empty());
}

// The rule: clippy runs on the crate of every patched file, and every
// finding inside a changed function lands within that function's span.
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
    let done = clippy_in(&dir, tree)?;
    Ok(done
        .iter()
        .filter_map(|finding| escaped(&built, tree, &dir, finding))
        .collect())
}

fn clippy_in(dir: &Path, tree: &Path) -> Result<Vec<Finding>, String> {
    let conf = tree.join("target").join("hyperray");
    if let Err(error) = std::fs::create_dir_all(&conf) {
        return Err(format!("{}: {error}", conf.display()));
    }
    match run(dir, &conf) {
        Err(error) => Err(format!("clippy did not run: {error}")),
        Ok(done) if done.exit_code != Some(0) => Err("clippy failed".to_string()),
        Ok(done) => Ok(done.findings),
    }
}

// Clippy prints a path relative to the crate; the manifest's is relative
// to the tree root, so one is rebased onto the other before comparing.
fn escaped(built: &Manifest, tree: &Path, dir: &Path, finding: &Finding) -> Option<String> {
    let in_tree = dir.join(&finding.path);
    let relative = in_tree.strip_prefix(tree).ok()?.display().to_string();
    let owner = built
        .functions
        .iter()
        .find(|f| owns(f, &relative, finding))?;
    match finding.line_end <= owner.end_line {
        true => None,
        false => Some(format!("{}: {} escapes its span", owner.name, finding.lint)),
    }
}

fn owns(function: &Located, path: &str, finding: &Finding) -> bool {
    let span = function.start_line..=function.end_line;
    function.path == path && span.contains(&finding.line_start)
}

fn tree_for(sources: &Path, name: &str) -> Option<PathBuf> {
    let tree = sources.join(name.split('-').next()?);
    match tree.is_dir() {
        true => Some(tree),
        false => None,
    }
}
