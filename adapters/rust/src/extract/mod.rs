// EXTRACT: pass 1 reads the patch text alone; pass 2 reads only the files
// the patch names and what Charon says about them. No item name is ever
// built here; the compiler wrote them and `seen` reads them back.

mod charon;
mod function;
mod hunk;
mod join;
mod locate;
mod names;
mod patch;
mod reader;
mod refusal;
mod seen;
mod ullbc;

pub use charon::{module_of, run, Run, Scope};
pub use hunk::Hunk;
pub use join::{join, Joined, Status};
pub use locate::{manifest, Located, Manifest};
pub use patch::FileChange;
pub use reader::Opened;
pub use refusal::{refusals_in, Refusal};
pub use seen::{seen_in, Body, Seen};

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
