// One MODEL row joins a manifest function to its exact compiler item. A
// missing or ambiguous join is an extraction error, not a partial row.

use super::{ownership, Error, ItemAnalysis, Row};
use crate::extract::{Joined, Status as JoinStatus};

pub fn rows(manifest: &[Joined], items: &[ItemAnalysis]) -> Result<Vec<Row>, Error> {
    manifest.iter().map(|entry| row(entry, items)).collect()
}

fn row(entry: &Joined, items: &[ItemAnalysis]) -> Result<Row, Error> {
    let item_path = entry
        .item_path
        .as_deref()
        .ok_or_else(|| row_error(entry, "stage 1 did not name a compiler item".to_string()))?;
    if entry.status != JoinStatus::Extracted {
        return Err(row_error(
            entry,
            "stage 1 did not extract a compiler body".to_string(),
        ));
    }
    let roots: Vec<&ItemAnalysis> = items
        .iter()
        .filter(|item| item.name == item_path && item.path == entry.path)
        .collect();
    let [root] = roots.as_slice() else {
        return Err(row_error(entry, root_error(item_path, roots.len())));
    };
    Ok(analyzed(entry, root, items))
}

fn analyzed(entry: &Joined, root: &ItemAnalysis, items: &[ItemAnalysis]) -> Row {
    let mut loops = root.loops.clone();
    for item in ownership::descendants(&root.name, items) {
        loops.extend(item.loops.clone());
    }
    Row {
        path: entry.path.clone(),
        name: entry.name.clone(),
        start_line: entry.start_line,
        end_line: entry.end_line,
        inputs: root.inputs.clone(),
        loops,
    }
}

fn row_error(entry: &Joined, reason: String) -> Error {
    Error::Row {
        path: entry.path.clone(),
        name: entry.name.clone(),
        reason,
    }
}

fn root_error(item: &str, count: usize) -> String {
    format!("compiler item {item} has {count} Stage 3 analyses")
}
