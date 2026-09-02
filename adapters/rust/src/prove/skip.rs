// The skip table Kani prints under "did not generate automatic harnesses":
// a header row `| Crate | Skipped Function | Reason for Skipping |`, then
// one `| crate | function | reason |` row per function, closed by `+---`.

use serde::Serialize;

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
pub struct Skipped {
    pub function: String,
    pub reason: String,
}

pub fn skipped_in(log: &str) -> Vec<Skipped> {
    let mut rows = Vec::new();
    let mut in_table = false;
    for line in log.lines() {
        if line.contains("| Skipped Function") {
            in_table = true;
            continue;
        }
        if in_table && line.starts_with('+') && !line.starts_with("+=") {
            break;
        }
        if in_table {
            rows.extend(row(line));
        }
    }
    rows
}

fn row(line: &str) -> Option<Skipped> {
    let mut cells = line.strip_prefix('|')?.strip_suffix('|')?.split('|');
    let _crate = cells.next()?;
    let function = cells.next()?.trim();
    let reason = cells.next()?.trim();
    Some(Skipped {
        function: function.to_string(),
        reason: reason.to_string(),
    })
}
