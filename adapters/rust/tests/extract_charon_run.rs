mod common;

use hyperray_rust::extract::{
    change, crate_dir, join, manifest, run, seen_in, Global, Joined, Read, Status,
};
use std::collections::BTreeSet;
use std::path::{Path, PathBuf};

struct Range {
    path: String,
    first: u32,
    last: u32,
}

struct Checked {
    joined: Vec<Joined>,
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
        let Some(crate_name) = dir.file_name().and_then(|n| n.to_str()) else {
            panic!("{}", dir.display());
        };
        let output = work.join(format!("{crate_name}.ullbc"));
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
        joined: join(&built.functions, &read.functions),
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
// with Charon's own reason, or reported missing; an extracted function
// with a plain name carries Charon's item path, ending in its own name;
// every global inside a patched range carries its source text.
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
        assert!(!checked.joined.is_empty(), "{name}");
        for function in &checked.joined {
            match &function.status {
                Status::Refused(reason) => assert!(!reason.is_empty()),
                Status::FileNotSeen => panic!("{name}: a patched file never reached Charon"),
                Status::Extracted | Status::Missing => {}
            }
            if let Some(item_path) = &function.item_path {
                assert!(item_path.ends_with(&function.name), "{item_path}");
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
