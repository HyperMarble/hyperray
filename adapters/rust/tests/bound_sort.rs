mod common;

use hyperray_rust::bound::{read, sort_all, Output, Pile, Sorted};
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

// Phase B rule: every limit is an evaluated scalar with a file and line
// that Charon itself listed; a body with no comparison has no limit.
fn check_limits(rows: &[&Sorted]) -> usize {
    let mut cited = 0;
    for limit in rows.iter().flat_map(|r| r.limits.iter()) {
        assert!(limit.value.parse::<i128>().is_ok(), "{}", limit.value);
        assert!(limit.line > 0 && !limit.path.is_empty(), "{limit:?}");
        assert!(limit.compared_with.is_comparison(), "{limit:?}");
        cited += 1;
    }
    cited
}

fn report(name: &str, rows: &[&Sorted], cited: usize) {
    eprintln!(
        "{name}: {} touched; none {} loop {} fixed {} sized {} unbuildable {}; limits cited {cited}",
        rows.len(),
        count(rows, |p| *p == Pile::NoBound),
        count(rows, |p| *p == Pile::Loop),
        count(rows, |p| *p == Pile::FixedWidth),
        count(rows, |p| *p == Pile::Sized),
        count(rows, |p| matches!(p, Pile::Unbuildable { .. })),
    );
}

fn output_for(tree: &Path, name: &str) -> Option<Output> {
    let ullbc = tree.join("target").join("hyperray").join("charon.ullbc");
    let Ok(file) = std::fs::File::open(&ullbc) else {
        eprintln!("skipped: {} not on disk", ullbc.display());
        return None;
    };
    let Ok(output) = read(file) else {
        panic!("{name}: charon output did not parse for stage 3");
    };
    Some(output)
}

// The rule (stage3.md Phase A and B) on every fixture whose Charon
// output is on disk.
#[test]
fn every_function_charon_saw_lands_in_one_pile_with_cited_limits() {
    let Some(sources) = common::dir("HYPERRAY_FIXTURE_SRC") else {
        return;
    };
    for (name, text) in common::patches() {
        let Some(tree) = name.split('-').next().map(|n| sources.join(n)) else {
            continue;
        };
        let Some(output) = output_for(&tree, &name) else {
            continue;
        };
        let all = sort_all(&output);
        let rows: Vec<&Sorted> = all.iter().filter(|s| touched(s, &text, &tree)).collect();
        assert!(!rows.is_empty(), "{name}");
        check_piles(&rows);
        let cited = check_limits(&rows);
        report(&name, &rows, cited);
    }
}
