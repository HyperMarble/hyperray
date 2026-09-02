// A fixture test: the real noodles-296 patch must yield the ranges we
// measured by hand on 2026-09-02. No invented diff.

use hyperray_rust::extract::patch::changed_files;
use std::path::Path;

fn fixture_patch() -> String {
    let path =
        Path::new(env!("CARGO_MANIFEST_DIR")).join("../../fixtures/noodles-296/solution.patch");
    std::fs::read_to_string(path).expect("fixture patch must exist")
}

#[test]
fn noodles_patch_touches_fourteen_files() {
    let files = changed_files(&fixture_patch());
    assert_eq!(files.len(), 14);
}

#[test]
fn builder_file_is_wholly_new_and_detect_sits_in_its_range() {
    let files = changed_files(&fixture_patch());
    let builder = files
        .iter()
        .find(|f| f.path == "noodles-util/src/alignment/async/io/indexed_reader/builder.rs")
        .expect("builder.rs must be in the patch");

    assert_eq!(builder.added.len(), 1, "a new file is one range");
    let range = &builder.added[0];
    assert_eq!(*range.start(), 1);
    assert!(range.contains(&201), "MAX_DETECTION_PREFIX_LEN at line 201");
    assert!(range.contains(&322), "the seek subtraction at line 322");
}

#[test]
fn sync_builder_has_two_separate_hunks() {
    let files = changed_files(&fixture_patch());
    let sync = files
        .iter()
        .find(|f| f.path == "noodles-util/src/alignment/io/reader/builder.rs")
        .expect("sync builder.rs must be in the patch");

    assert_eq!(
        sync.added.len(),
        2,
        "the CRAM version arm and the helper fn"
    );
}
