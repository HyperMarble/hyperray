// Stage 1 requires a compiler answer for every function touched by a patch.

mod common;
#[path = "extract_mir/check.rs"]
mod extract_mir_check;
#[path = "extract_mir/compile.rs"]
mod extract_mir_compile;
#[path = "extract_mir/read.rs"]
mod extract_mir_read;

use crate::extract_mir_check::{checked, tree_for};
use crate::extract_mir_read::read_all;

#[test]
fn every_patched_function_has_a_status_from_the_compiler() {
    let (Some(driver), Some(sources)) = (
        common::dir("HYPERRAY_MIR_DUMP"),
        common::dir("HYPERRAY_FIXTURE_SRC"),
    ) else {
        return;
    };
    let mut ran = 0;
    let mut failures: Vec<String> = Vec::new();
    for (name, text) in common::patches() {
        let Some(tree) = tree_for(&sources, &name) else {
            continue;
        };
        match read_all(&driver, &tree, &text) {
            Ok(read) => checked(&name, &read, &mut ran),
            Err(reason) => failures.push(reason),
        }
    }
    assert!(failures.is_empty(), "{}", failures.join("\n"));
    assert!(ran > 0, "no fixture had a source tree");
}
