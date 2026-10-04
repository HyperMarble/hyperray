// A source edit that emits the same instructions is not a change.
// A source edit that emits different instructions is.
use crate::changed::{differing, Difference};
use crate::compile::compile;
use std::path::PathBuf;

const BASE: &str = r#"#[no_mangle] #[inline(never)] pub extern "C" fn half(n: u64) -> u64 { n / 2 }
#[no_mangle] #[inline(never)] pub extern "C" fn step(n: u64) -> u64 { n + 1 }
fn main() { println!("{} {}", half(std::hint::black_box(4)), step(1)); }
"#;

fn scratch(name: &str) -> PathBuf {
    let dir = std::env::temp_dir().join("hyperray_rust_changed");
    std::fs::create_dir_all(&dir).expect("scratch directory");
    dir.join(name)
}

fn build(source_text: &str, stem: &str) -> Vec<u8> {
    let source = scratch(&format!("{stem}.rs"));
    let output = scratch(&format!("{stem}.bin"));
    std::fs::write(&source, source_text).expect("write source");
    compile(&source, &output, "aarch64-apple-darwin").expect("source must compile");
    std::fs::read(&output).expect("read binary")
}

#[test]
fn a_rewrite_with_identical_instructions_is_not_a_change() {
    let before = build(BASE, "same_before");
    let after = build(&BASE.replace("n / 2", "n >> 1"), "same_after");
    let names = vec!["half".to_string()];
    assert_eq!(differing(&before, &after, &names).expect("compare"), vec![]);
}

#[test]
fn a_rewrite_with_different_instructions_is_a_change() {
    let before = build(BASE, "diff_before");
    let after = build(&BASE.replace("n + 1", "n + 2"), "diff_after");
    let names = vec!["step".to_string()];
    let found = differing(&before, &after, &names).expect("compare");
    match found.as_slice() {
        [Difference::Changed { name, before, after }] => {
            assert_eq!(name, "step");
            assert_ne!(before, after, "the recorded bytes must differ");
        }
        other => panic!("one changed function expected, got {other:?}"),
    }
}
