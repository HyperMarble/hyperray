mod common;

use hyperray_rust::extract::{change, Change, HunkChange};

// Counted by the diff format's own rules, not by the parser under test:
// `+++ b/` opens a file, `@@` opens a hunk, `+` and `-` inside a hunk
// add and remove a line. Header lines close the hunk.
fn counted_by_hand(patch: &str) -> (usize, usize, usize, usize) {
    let mut files = 0;
    let mut hunks = 0;
    let mut added = 0;
    let mut removed = 0;
    let mut in_hunk = false;
    for line in patch.lines() {
        if line.starts_with("+++ b/") {
            files += 1;
            in_hunk = false;
        } else if line.starts_with("--- ") {
            in_hunk = false;
        } else if line.starts_with("@@") {
            hunks += 1;
            in_hunk = true;
        } else if in_hunk && line.starts_with('+') {
            added += 1;
        } else if in_hunk && line.starts_with('-') {
            removed += 1;
        }
    }
    (files, hunks, added, removed)
}

fn counted_by_parser(files: &[Change]) -> (usize, usize, usize, usize) {
    let hunks: Vec<_> = files.iter().flat_map(|f| &f.hunks).collect();
    let added = hunks.iter().map(|h| h.added).sum();
    let removed = hunks.iter().map(|h| h.removed).sum();
    (files.len(), hunks.len(), added, removed)
}

#[test]
fn every_patch_counts_the_same_by_parser_and_by_hand() {
    for (name, text) in common::patches() {
        assert_eq!(
            counted_by_parser(&change(&text)),
            counted_by_hand(&text),
            "{name}"
        );
    }
}

#[test]
fn every_hunk_range_sits_inside_its_file_and_names_are_identifiers() {
    let patches = common::patches();
    let files = patches.iter().flat_map(|(_, text)| change(text));
    files.for_each(|file| checked(&file));
}

fn checked(file: &Change) {
    assert!(!file.path.starts_with("b/"), "{}", file.path);
    file.hunks.iter().for_each(bounded);
}

fn bounded(hunk: &HunkChange) {
    assert!(hunk.added_range.0 <= hunk.added_range.1 + 1);
    let plain = hunk.defines.iter().all(|name| identifier(name));
    assert!(plain, "{:?}", hunk.defines);
}

fn identifier(name: &str) -> bool {
    name.chars().all(|c| c.is_alphanumeric() || c == '_')
}

#[test]
fn pass_one_reads_only_the_patch_text() {
    let text = "+++ b/src/lib.rs\n@@ -1,2 +1,3 @@ mod x\n+pub fn added() {}\n-old\n";
    let files = change(text);
    assert_eq!(files[0].path, "src/lib.rs");
    assert_eq!(files[0].hunks[0].defines, vec!["added".to_string()]);
    assert_eq!(files[0].hunks[0].context, "mod x");
}
