// EXTRACT: pass 1 reads the patch text alone; pass 2 compiles the crate and
// reads what the compiler says about every item it built. No item name is
// ever built here; the compiler wrote them and `seen` reads them back.

mod driver;
mod function;
mod global;
mod hunk;
mod join;
mod locate;
mod mir;
mod names;
mod patch;
mod reader;
mod seen;
mod workspace;

pub use driver::{run, Run};
pub use global::Global;
pub use hunk::Hunk;
pub use join::{join, Joined, Status};
pub use locate::{manifest, Located, Manifest};
pub use patch::FileChange;
pub use reader::Opened;
pub use seen::{seen_in, Body, Read, Seen};
pub use workspace::crate_dir;

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
