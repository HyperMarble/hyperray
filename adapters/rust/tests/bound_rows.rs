mod common;

use hyperray_rust::bound::{read, rows, sort_all, Bound, Limit, Row, Sorted};
use hyperray_rust::extract::{change, join, manifest, seen_in, Joined, Seen};
use std::path::Path;

struct Loaded {
    joined: Vec<Joined>,
    sorted: Vec<Sorted>,
}

fn load(tree: &Path, patch: &str) -> Option<Loaded> {
    let files = common::ullbc_files(tree);
    if files.is_empty() {
        return None;
    }
    let mut seen: Vec<Seen> = Vec::new();
    let mut sorted = Vec::new();
    for path in &files {
        seen.extend(seen_in(std::fs::File::open(path).ok()?).ok()?.functions);
        sorted.extend(sort_all(&read(std::fs::File::open(path).ok()?).ok()?));
    }
    let built = manifest(tree, &change(patch)).ok()?;
    Some(Loaded {
        joined: join(&built.functions, &seen),
        sorted,
    })
}

fn check_limits(limits: &[Limit]) {
    for l in limits {
        assert!(!l.path.is_empty() && l.line > 0, "{l:?}");
        assert!(l.value.parse::<i128>().is_ok(), "{l:?}");
        assert!(l.compared_with.is_comparison(), "{l:?}");
    }
}

// stage3.md Phase D test.
fn check(name: &str, joined: &[Joined], all: &[Row]) {
    assert_eq!(all.len(), joined.len(), "{name}");
    for (row, j) in all.iter().zip(joined) {
        assert_eq!((&row.path, row.start_line), (&j.path, j.start_line));
        match &row.bound {
            Bound::NotInCharon { reason } => assert!(!reason.is_empty(), "{name} {}", row.name),
            Bound::Loop { limits, reason, .. } => {
                assert!(!limits.is_empty() || reason.is_some(), "{name}")
            }
            Bound::Sized { limits, .. } => check_limits(limits),
            Bound::None | Bound::FixedWidth { .. } | Bound::Unbuildable { .. } => {}
        }
    }
}

#[test]
fn bound_json_has_one_row_per_manifest_function() {
    let Some(sources) = common::dir("HYPERRAY_FIXTURE_SRC") else {
        return;
    };
    for (name, text) in common::patches() {
        let Some(tree) = common::tree_for(&sources, &name) else {
            continue;
        };
        let Some(loaded) = load(&tree, &text) else {
            eprintln!("skipped: {name} has no Charon output on disk");
            continue;
        };
        let all = rows(&loaded.joined, &loaded.sorted);
        check(&name, &loaded.joined, &all);
        let Ok(json) = serde_json::to_string(&all) else {
            panic!("{name}: rows did not serialize");
        };
        let kinds = json.matches("\"kind\":\"").count();
        eprintln!("{name}: {} rows, {kinds} tagged", all.len());
    }
}
