mod common;

use hyperray_rust::bound::{read, sort_all, Pile, Sorted};
use hyperray_rust::extract::{change, manifest};
use std::path::Path;

fn touched(sorted: &Sorted, patch: &str, root: &Path) -> bool {
    let Ok(built) = manifest(root, &change(patch)) else {
        return false;
    };
    built
        .functions
        .iter()
        .any(|f| f.path == sorted.path && f.start_line == sorted.start_line)
}

fn count(rows: &[&Sorted], want: fn(&Pile) -> bool) -> usize {
    rows.iter().filter(|r| want(&r.pile)).count()
}

fn check(name: &str, rows: &[&Sorted]) {
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
    eprintln!(
        "{name}: {} touched; none {} loop {} fixed {} sized {} unbuildable {}",
        rows.len(),
        count(rows, |p| *p == Pile::NoBound),
        count(rows, |p| *p == Pile::Loop),
        count(rows, |p| *p == Pile::FixedWidth),
        count(rows, |p| *p == Pile::Sized),
        count(rows, |p| matches!(p, Pile::Unbuildable { .. })),
    );
}

// The rule (stage3.md Phase A): every function has exactly one pile; a
// function whose body has a cycle is in the loop pile and nowhere else;
// a pile-4 row names the input and the type kind that put it there.
#[test]
fn every_function_charon_saw_lands_in_one_pile() {
    let Some(sources) = common::dir("HYPERRAY_FIXTURE_SRC") else {
        return;
    };
    let mut ran = 0;
    for (name, text) in common::patches() {
        let Some(tree) = name.split('-').next().map(|n| sources.join(n)) else {
            continue;
        };
        let ullbc = tree.join("target").join("hyperray").join("charon.ullbc");
        let Ok(file) = std::fs::File::open(&ullbc) else {
            eprintln!("skipped: {} not on disk", ullbc.display());
            continue;
        };
        let Ok(output) = read(file) else {
            panic!("{name}: charon output did not parse for stage 3");
        };
        let all = sort_all(&output);
        let rows: Vec<&Sorted> = all.iter().filter(|s| touched(s, &text, &tree)).collect();
        assert!(!rows.is_empty(), "{name}");
        check(&name, &rows);
        ran += 1;
    }
    eprintln!("stage 3 phase A ran on {ran} fixture(s)");
}
