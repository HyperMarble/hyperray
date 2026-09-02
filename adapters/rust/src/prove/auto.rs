// Kani's `autoharness` output read back: one verdict per function Kani
// chose, and one reason per function it skipped. Every line shape here
// was copied from a real run; nothing is inferred.

use super::report::{results_in, HarnessResult};
use super::skip::{skipped_in, Skipped};
use serde::Serialize;

#[derive(Debug, Serialize, PartialEq, Eq)]
pub struct Auto {
    pub verdicts: Vec<HarnessResult>,
    pub skipped: Vec<Skipped>,
}

pub fn auto_in(log: &str) -> Auto {
    Auto {
        verdicts: results_in(&as_harness_lines(log)),
        skipped: skipped_in(log),
    }
}

// `Autoharness: Checking function <path> against all possible inputs...`
// is rewritten to the `Checking harness <path>...` line `report` reads,
// so both runs share one result reader.
fn as_harness_lines(log: &str) -> String {
    let mut out = String::with_capacity(log.len());
    for line in log.lines() {
        match function_name(line) {
            Some(name) => out.push_str(&format!("Checking harness {name}...\n")),
            None => {
                out.push_str(line);
                out.push('\n');
            }
        }
    }
    out
}

fn function_name(line: &str) -> Option<&str> {
    line.strip_prefix("Autoharness: Checking function ")?
        .strip_suffix(" against all possible inputs...")
}
