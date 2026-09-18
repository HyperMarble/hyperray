// Stage 3 joins exact compiler ownership to each manifest function.
// A sibling item must never change the function row.

use crate::bound_case;
use hyperray_rust::bound::{analyze_dump, rows, Row};
use hyperray_rust::extract::{Joined, Status as ExtractStatus};
use hyperray_rust::mir::{Dump, SCHEMA_VERSION};

#[test]
fn a_row_keeps_root_and_descendant_loops_but_not_siblings() -> Result<(), Box<dyn std::error::Error>>
{
    let root = bound_case::counted_loop();
    let mut child = bound_case::iterator_loop();
    child.name = "crate::counted::{closure#0}".to_string();
    child.parent = Some(root.name.clone());
    let mut sibling = bound_case::iterator_loop();
    sibling.name = "crate::sibling".to_string();
    sibling.parent = Some("crate".to_string());
    let dump = Dump {
        schema_version: SCHEMA_VERSION,
        crate_name: "crate".to_string(),
        items: vec![root, child, sibling],
        instances: Vec::new(),
    };
    let manifest = vec![joined("crate::counted")];
    let result = rows(&manifest, &analyze_dump(&dump)?)?;

    assert_eq!(result.len(), 1);
    assert_eq!(result[0].loops.len(), 2);
    let json = serde_json::to_string(&result)?;
    let decoded: Vec<Row> = serde_json::from_str(&json)?;
    assert_eq!(decoded, result);
    Ok(())
}

fn joined(item: &str) -> Joined {
    Joined {
        path: "src/lib.rs".to_string(),
        name: "counted".to_string(),
        start_line: 1,
        end_line: 8,
        item_path: Some(item.to_string()),
        status: ExtractStatus::Extracted,
    }
}
