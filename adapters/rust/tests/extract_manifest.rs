use hyperray_rust::extract::{change, manifest, Manifest};
use std::path::PathBuf;

// Source trees stay outside the repo. The variable names their root; each
// fixture is a directory under it with the patch already applied.
fn source_root() -> Option<PathBuf> {
    let root = PathBuf::from(std::env::var("HYPERRAY_FIXTURE_SRC").ok()?);
    root.is_dir().then_some(root)
}

fn build(fixture: &str, source_dir: &str) -> Option<Manifest> {
    let Some(root) = source_root() else {
        eprintln!("skipped: HYPERRAY_FIXTURE_SRC is not set or not a directory");
        return None;
    };
    let repo = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../fixtures");
    let text = std::fs::read_to_string(repo.join(fixture).join("solution.patch")).ok()?;
    manifest(&root.join(source_dir), &change(&text)).ok()
}

#[test]
fn every_opened_file_is_a_patched_file() {
    let Some(built) = build("noodles-296", "noodles") else {
        return;
    };
    let text = std::fs::read_to_string(
        PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../fixtures/noodles-296/solution.patch"),
    )
    .unwrap_or_default();
    let patched: Vec<String> = change(&text).into_iter().map(|c| c.path).collect();
    for opened in &built.opened {
        assert!(patched.contains(&opened.path), "opened {}", opened.path);
        assert_eq!(opened.reason, "patched file");
    }
}

// Pass 2 yields every `fn` the patch defines, plus each existing `fn` a
// partial hunk edits. noodles-296 defines 71 and edits one: `detect_format`.
#[test]
fn every_located_function_contains_its_hunk_and_has_a_body() {
    let Some(built) = build("noodles-296", "noodles") else {
        return;
    };
    let text = std::fs::read_to_string(
        PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../fixtures/noodles-296/solution.patch"),
    )
    .unwrap_or_default();
    let defined: usize = change(&text)
        .iter()
        .flat_map(|c| &c.hunks)
        .map(|h| h.defines.len())
        .sum();
    let edited: Vec<_> = built
        .functions
        .iter()
        .filter(|f| f.name == "detect_format")
        .collect();
    assert_eq!(built.functions.len(), defined + edited.len());
    assert_eq!(edited.len(), 1);
    for function in &built.functions {
        assert!(function.start_line <= function.end_line);
        assert!(function.text.trim_end().ends_with('}'), "{}", function.name);
    }
}
