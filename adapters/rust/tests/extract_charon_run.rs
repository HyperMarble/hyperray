mod common;

use hyperray_rust::extract::{
    change, crate_dir, join, manifest, run, seen_in, Global, Read, Status,
};
use std::collections::BTreeSet;
use std::path::{Path, PathBuf};

struct Range {
    path: String,
    first: u32,
    last: u32,
}

struct Checked {
    statuses: Vec<Status>,
    globals: Vec<Global>,
    ranges: Vec<Range>,
}

fn ranges_of(patch: &str) -> Vec<Range> {
    change(patch)
        .iter()
        .flat_map(|c| {
            c.hunks.iter().map(move |h| Range {
                path: c.path.clone(),
                first: h.added_range.0,
                last: h.added_range.1,
            })
        })
        .collect()
}

fn read_all(charon: &Path, root: &Path, patch: &str) -> Checked {
    let ranges = ranges_of(patch);
    let Ok(built) = manifest(root, &change(patch)) else {
        panic!("manifest failed for {}", root.display());
    };
    let crates: BTreeSet<PathBuf> = built
        .functions
        .iter()
        .filter_map(|f| crate_dir(root, &f.path))
        .collect();
    let work = root.join("target").join("hyperray");
    assert!(std::fs::create_dir_all(&work).is_ok(), "{}", work.display());
    let mut read = Read {
        functions: Vec::new(),
        globals: Vec::new(),
    };
    for dir in &crates {
        let output = work.join("charon.ullbc");
        let done = run(charon, dir, &output);
        assert_eq!(done.exit_code, Some(0), "{}\n{}", dir.display(), done.log);
        let Ok(file) = std::fs::File::open(&output) else {
            panic!("{}", output.display());
        };
        let Ok(part) = seen_in(file) else {
            panic!("charon output did not parse");
        };
        read.functions.extend(part.functions);
        read.globals.extend(part.globals);
    }
    Checked {
        statuses: join(&built.functions, &read.functions)
            .into_iter()
            .map(|j| j.status)
            .collect(),
        globals: read.globals,
        ranges,
    }
}

fn in_patch(ranges: &[Range], global: &Global) -> bool {
    ranges
        .iter()
        .any(|r| r.path == global.path && (r.first..=r.last).contains(&global.start_line))
}

// The rule: every function the patch touches is extracted, or refused
// with Charon's own reason, or reported missing; every global inside a
// patched range carries its source text. No fixture is named here.
#[test]
fn every_patched_function_and_global_has_a_status_from_charon() {
    let (Some(charon), Some(sources)) = (
        common::dir("HYPERRAY_CHARON"),
        common::dir("HYPERRAY_FIXTURE_SRC"),
    ) else {
        return;
    };
    let mut ran = 0;
    for (name, text) in common::patches() {
        let Some(tree) = name.split('-').next().map(|n| sources.join(n)) else {
            continue;
        };
        if !tree.is_dir() {
            continue;
        }
        let checked = read_all(&charon, &tree, &text);
        assert!(!checked.statuses.is_empty(), "{name}");
        for status in &checked.statuses {
            match status {
                Status::Refused(reason) => assert!(!reason.is_empty()),
                Status::FileNotSeen => panic!("{name}: a patched file never reached Charon"),
                Status::Extracted | Status::Missing => {}
            }
        }
        for global in checked
            .globals
            .iter()
            .filter(|g| in_patch(&checked.ranges, g))
        {
            assert!(global.source_text.is_some(), "{name}: {}", global.path);
        }
        ran += 1;
    }
    assert!(ran > 0, "no fixture had a source tree");
}
