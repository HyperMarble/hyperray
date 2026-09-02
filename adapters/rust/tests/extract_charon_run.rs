mod common;

use hyperray_rust::extract::{change, crate_dir, join, manifest, run, seen_in, Status};
use std::collections::BTreeSet;
use std::path::{Path, PathBuf};

fn statuses(charon: &Path, root: &Path, patch: &str) -> Vec<Status> {
    let Ok(built) = manifest(root, &change(patch)) else {
        return Vec::new();
    };
    let crates: BTreeSet<PathBuf> = built
        .functions
        .iter()
        .filter_map(|f| crate_dir(root, &f.path))
        .collect();
    let work = root.join("target").join("hyperray");
    assert!(std::fs::create_dir_all(&work).is_ok(), "{}", work.display());
    let mut seen = Vec::new();
    for dir in &crates {
        let output = work.join("charon.ullbc");
        let done = run(charon, dir, &output);
        assert_eq!(done.exit_code, Some(0), "{}\n{}", dir.display(), done.log);
        let Ok(file) = std::fs::File::open(&output) else {
            panic!("{}", output.display());
        };
        seen.extend(seen_in(file).unwrap_or_default());
    }
    join(&built.functions, &seen)
        .into_iter()
        .map(|j| j.status)
        .collect()
}

// The rule: every function the patch touches is extracted, or refused
// with Charon's own reason, or reported missing. Nothing is dropped, and
// no fixture is named here.
#[test]
fn every_patched_function_has_a_status_from_charon() {
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
        let all = statuses(&charon, &tree, &text);
        assert!(!all.is_empty(), "{name}");
        for status in &all {
            match status {
                Status::Refused(reason) => assert!(!reason.is_empty()),
                Status::FileNotSeen => panic!("{name}: a patched file never reached Charon"),
                Status::Extracted | Status::Missing => {}
            }
        }
        ran += 1;
    }
    assert!(ran > 0, "no fixture had a source tree");
}
