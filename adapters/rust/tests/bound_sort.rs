mod common;

use hyperray_rust::bound::{read, sort_all, Pile, Sorted};
use hyperray_rust::extract::{change, manifest};
use std::path::Path;

// One Charon row per manifest function: the first whose span contains
// the start line (Charon adds closure/derive rows at the same line).
fn touched<'a>(all: &'a [Sorted], patch: &str, root: &Path) -> Vec<&'a Sorted> {
    let Ok(built) = manifest(root, &change(patch)) else {
        return Vec::new();
    };
    let contains = |s: &&Sorted, path: &str, line: u32| {
        s.path == path && s.start_line <= line && line <= s.end_line
    };
    built
        .functions
        .iter()
        .filter_map(|f| all.iter().find(|s| contains(s, &f.path, f.start_line)))
        .collect()
}

fn sorted_for(tree: &Path) -> Vec<Sorted> {
    let mut all = Vec::new();
    for path in common::ullbc_files(tree) {
        let Ok(file) = std::fs::File::open(&path) else {
            continue;
        };
        let Ok(output) = read(file) else {
            panic!("{}: charon output did not parse", path.display());
        };
        all.extend(sort_all(&output));
    }
    all
}

// Phase A rule: a body with a cycle is in the loop pile and nowhere
// else; a pile-4 row names the input and the type kind that put it there.
fn check_piles(rows: &[&Sorted]) {
    for row in rows {
        let at = format!("{}:{}", row.path, row.start_line);
        if row.charon_loop {
            assert_eq!(row.pile, Pile::Loop, "{at}");
        }
        if let Pile::Unbuildable { input, kind } = &row.pile {
            assert!(*input < row.inputs.len(), "{at}");
            assert!(!kind.is_empty(), "{at}");
        }
    }
}

#[test]
fn every_function_charon_saw_lands_in_one_pile() {
    let Some(sources) = common::dir("HYPERRAY_FIXTURE_SRC") else {
        return;
    };
    for (name, text) in common::patches() {
        let Some(tree) = common::tree_for(&sources, &name) else {
            continue;
        };
        let all = sorted_for(&tree);
        if all.is_empty() {
            eprintln!("skipped: {name} has no Charon output on disk");
            continue;
        }
        let rows = touched(&all, &text, &tree);
        assert!(!rows.is_empty(), "{name}");
        check_piles(&rows);
        let loops = rows.iter().filter(|r| r.pile == Pile::Loop).count();
        let limits: usize = rows.iter().map(|r| r.limits.len()).sum();
        eprintln!(
            "{name}: {} touched, {loops} loop, {limits} limits",
            rows.len()
        );
    }
}
