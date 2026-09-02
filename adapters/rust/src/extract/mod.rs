// EXTRACT pass 1: everything the patch text says, and nothing the
// repository says. `change` is the only public entry.

mod hunk;
mod names;
mod patch;

pub use hunk::Hunk;
pub use patch::FileChange;

use serde::Serialize;

#[derive(Debug, Serialize, PartialEq, Eq)]
pub struct HunkChange {
    pub added_range: (u32, u32),
    pub added: usize,
    pub removed: usize,
    pub context: String,
    pub defines: Vec<String>,
}

#[derive(Debug, Serialize, PartialEq, Eq)]
pub struct Change {
    pub path: String,
    pub hunks: Vec<HunkChange>,
}

pub fn change(patch_text: &str) -> Vec<Change> {
    patch::parse(patch_text)
        .into_iter()
        .map(|file| Change {
            path: file.path,
            hunks: file.hunks.iter().map(hunk_change).collect(),
        })
        .collect()
}

fn hunk_change(hunk: &Hunk) -> HunkChange {
    HunkChange {
        added_range: hunk.added_range(),
        added: hunk.added.len(),
        removed: hunk.removed.len(),
        context: hunk.context.clone(),
        defines: names::defined_in(hunk),
    }
}
