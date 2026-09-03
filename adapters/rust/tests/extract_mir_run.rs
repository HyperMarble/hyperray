// The rule stage 1 must keep: every function a patch touches gets an answer
// from the compiler, and a patched file always reaches it. No fixture name
// is known here; the walk is over whatever trees are on disk.
//
// There is no refusal to check for. A front end that cannot read a body is
// Hyperray's bug, not a status (design.md §2).

mod common;

use hyperray_rust::extract::{
    change, crate_dir, join, manifest, run, seen_in, Global, Joined, Read, Status,
};
use std::collections::BTreeSet;
use std::path::{Path, PathBuf};

pub struct Checked {
    pub joined: Vec<Joined>,
    pub globals: Vec<Global>,
}

fn read_all(driver: &Path, root: &Path, patch: &str) -> Result<Checked, String> {
    let changes = change(patch);
    let Ok(built) = manifest(root, &changes) else {
        return Err(format!("manifest failed for {}", root.display()));
    };
    let crates: BTreeSet<PathBuf> = built
        .functions
        .iter()
        .filter_map(|function| crate_dir(root, &function.path))
        .collect();
    let work = root.join("target").join("hyperray-mir");
    let read = read_crates(driver, &crates, &work)?;
    Ok(Checked {
        joined: join(&built.functions, &read.functions),
        globals: read.globals,
    })
}

fn read_crates(driver: &Path, crates: &BTreeSet<PathBuf>, work: &Path) -> Result<Read, String> {
    if let Err(error) = std::fs::create_dir_all(work) {
        return Err(format!("{}: {error}", work.display()));
    }
    for dir in crates {
        let done = run(driver, dir, work);
        if done.exit_code != Some(0) {
            return Err(format!("{}\n{}", dir.display(), done.log));
        }
        if done.dumps == 0 {
            return Err(format!("{}: the compiler wrote nothing", dir.display()));
        }
    }
    collect(&dumps_in(work))
}

fn collect(dumps: &[PathBuf]) -> Result<Read, String> {
    let mut read = Read {
        functions: Vec::new(),
        globals: Vec::new(),
    };
    for dump in dumps {
        let part = one_dump(dump)?;
        read.functions.extend(part.functions);
        read.globals.extend(part.globals);
    }
    Ok(read)
}

fn one_dump(dump: &Path) -> Result<Read, String> {
    let Ok(file) = std::fs::File::open(dump) else {
        return Err(format!("cannot open {}", dump.display()));
    };
    match seen_in(file) {
        Ok(read) => Ok(read),
        Err(error) => Err(format!("{}: {error}", dump.display())),
    }
}

// One file per crate, so the walk is over the directory, never a name the
// test builds.
fn dumps_in(work: &Path) -> Vec<PathBuf> {
    let Ok(entries) = std::fs::read_dir(work) else {
        return Vec::new();
    };
    entries
        .filter_map(|entry| entry.ok().map(|found| found.path()))
        .filter(|path| path.to_string_lossy().ends_with(".mir.json"))
        .collect()
}

fn answered(name: &str, function: &Joined) {
    let seen = function.status != Status::FileNotSeen;
    assert!(seen, "{name}: a patched file never reached the compiler");
    let path = function.item_path.clone().unwrap_or_default();
    let named = path.is_empty() || path.ends_with(&function.name);
    assert!(named, "{path} does not end in {}", function.name);
}

fn checked(name: &str, read: &Checked, ran: &mut u32) {
    assert!(!read.joined.is_empty(), "{name}: no rows");
    read.joined.iter().for_each(|row| answered(name, row));
    read.globals.iter().for_each(|global| placed(name, global));
    *ran += 1;
}

// A global carries where it is; its value is present only when the
// compiler could evaluate one, so the rule is on the place, not the value.
fn placed(name: &str, global: &Global) {
    assert!(!global.path.is_empty(), "{name}: a global with no file");
    assert!(global.start_line >= 1, "{name}: {}", global.path);
}

fn tree_for(sources: &Path, name: &str) -> Option<PathBuf> {
    let tree = sources.join(name.split('-').next()?);
    match tree.is_dir() {
        true => Some(tree),
        false => None,
    }
}

#[test]
fn every_patched_function_has_a_status_from_the_compiler() {
    let (Some(driver), Some(sources)) = (
        common::dir("HYPERRAY_MIR_DUMP"),
        common::dir("HYPERRAY_FIXTURE_SRC"),
    ) else {
        return;
    };
    let mut ran = 0;
    let mut failures: Vec<String> = Vec::new();
    for (name, text) in common::patches() {
        let Some(tree) = tree_for(&sources, &name) else {
            continue;
        };
        match read_all(&driver, &tree, &text) {
            Ok(read) => checked(&name, &read, &mut ran),
            Err(reason) => failures.push(reason),
        }
    }
    assert!(failures.is_empty(), "{}", failures.join("\n"));
    assert!(ran > 0, "no fixture had a source tree");
}
