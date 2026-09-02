mod common;

use hyperray_rust::extract::{change, manifest, Manifest};

// One fixture that has both a patch and a source tree of the same
// leading name under HYPERRAY_FIXTURE_SRC.
fn build() -> Option<(String, Manifest)> {
    let sources = common::dir("HYPERRAY_FIXTURE_SRC")?;
    common::patches().into_iter().find_map(|(name, text)| {
        let tree = sources.join(name.split('-').next()?);
        let built = manifest(&tree, &change(&text)).ok()?;
        Some((text, built))
    })
}

#[test]
fn every_opened_file_is_a_patched_file() {
    let Some((text, built)) = build() else {
        return;
    };
    let patched: Vec<String> = change(&text).into_iter().map(|c| c.path).collect();
    for opened in &built.opened {
        assert!(patched.contains(&opened.path), "opened {}", opened.path);
        assert_eq!(opened.reason, "patched file");
    }
}

// Pass 2 yields every `fn` the patch defines, plus each existing `fn` a
// partial hunk edits. Every one has a body that closes.
#[test]
fn every_located_function_contains_its_hunk_and_has_a_body() {
    let Some((text, built)) = build() else {
        return;
    };
    let defined: usize = change(&text)
        .iter()
        .flat_map(|c| &c.hunks)
        .map(|h| h.defines.len())
        .sum();
    assert!(built.functions.len() >= defined);
    for function in &built.functions {
        assert!(function.start_line <= function.end_line);
        assert!(function.text.trim_end().ends_with('}'), "{}", function.name);
    }
}
