mod common;

use hyperray_rust::extract::{change, Change};

fn totals(files: &[Change]) -> (usize, usize, usize, usize, usize) {
    let hunks: Vec<_> = files.iter().flat_map(|f| &f.hunks).collect();
    let added = hunks.iter().map(|h| h.added).sum();
    let removed = hunks.iter().map(|h| h.removed).sum();
    let defined = hunks.iter().map(|h| h.defines.len()).sum();
    (files.len(), hunks.len(), added, removed, defined)
}

// Counts measured with an independent line count on 2026-09-02:
// files, hunks, added lines, removed lines, `fn` definitions added.
#[test]
fn every_fixture_yields_the_measured_counts() {
    let expected = [
        ("alloy-ws-batch", (8, 24, 311, 28, 17)),
        ("noodles-296", (14, 16, 1052, 6, 71)),
        ("serde-json-1156", (5, 10, 481, 26, 37)),
        ("wasmtime-cfg", (2, 14, 212, 6, 6)),
    ];
    let patches = common::patches();
    if patches.is_empty() {
        return;
    }
    let seen: Vec<_> = patches
        .iter()
        .map(|(name, text)| (name.clone(), totals(&change(text))))
        .collect();
    let expected: Vec<_> = expected
        .iter()
        .map(|(name, counts)| (name.to_string(), *counts))
        .collect();
    assert_eq!(seen, expected);
}

#[test]
fn every_hunk_range_sits_inside_its_file_and_names_are_identifiers() {
    for (_, text) in common::patches() {
        for file in &change(&text) {
            assert!(!file.path.starts_with("b/"), "{}", file.path);
            for hunk in &file.hunks {
                assert!(hunk.added_range.0 <= hunk.added_range.1 + 1);
                for name in &hunk.defines {
                    assert!(name.chars().all(|c| c.is_alphanumeric() || c == '_'));
                }
            }
        }
    }
}

#[test]
fn pass_one_reads_only_the_patch_text() {
    let text = "+++ b/src/lib.rs\n@@ -1,2 +1,3 @@ mod x\n+pub fn added() {}\n-old\n";
    let files = change(text);
    assert_eq!(files[0].path, "src/lib.rs");
    assert_eq!(files[0].hunks[0].defines, vec!["added".to_string()]);
    assert_eq!(files[0].hunks[0].context, "mod x");
}
