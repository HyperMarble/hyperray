// Kani's regular output read back: one result per harness, with the
// check count, the failed checks, and the time. Every line shape here was
// copied from a real run; nothing is inferred.

use serde::Serialize;

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
pub struct HarnessResult {
    pub harness: String,
    pub passed: bool,
    pub checks: u32,
    pub failed_checks: Vec<String>,
    pub time_s: String,
}

pub fn results_in(log: &str) -> Vec<HarnessResult> {
    let mut found = Vec::new();
    let mut current: Option<HarnessResult> = None;
    for line in log.lines() {
        if let Some(name) = harness_name(line) {
            found.extend(current.take());
            current = Some(blank(name));
        }
        let closed = current.as_mut().is_some_and(|result| fill(result, line));
        if closed {
            found.extend(current.take());
        }
    }
    found.extend(current);
    found
}

fn blank(harness: &str) -> HarnessResult {
    HarnessResult {
        harness: harness.to_string(),
        passed: false,
        checks: 0,
        failed_checks: Vec::new(),
        time_s: String::new(),
    }
}

// `Checking harness harness::old_equals_new...`
fn harness_name(line: &str) -> Option<&str> {
    line.strip_prefix("Checking harness ")?.strip_suffix("...")
}

// ` ** 0 of 33 failed` / `Failed Checks: attempt to subtract with overflow`
// / `VERIFICATION:- SUCCESSFUL` / `Verification Time: 0.0247s`. Every
// block ends with the time line, so that line closes the result.
fn fill(result: &mut HarnessResult, line: &str) -> bool {
    let text = line.trim();
    if let Some(rest) = text.strip_prefix("** ") {
        result.checks = checks_total(rest).unwrap_or(0);
    } else if let Some(reason) = text.strip_prefix("Failed Checks: ") {
        result.failed_checks.push(reason.to_string());
    } else if text == "VERIFICATION:- SUCCESSFUL" {
        result.passed = true;
    } else if let Some(time) = text.strip_prefix("Verification Time: ") {
        result.time_s = time.trim_end_matches('s').to_string();
        return true;
    }
    false
}

// `1 of 33 failed` -> 33
fn checks_total(rest: &str) -> Option<u32> {
    let (_, tail) = rest.split_once(" of ")?;
    let (total, _) = tail.split_once(' ')?;
    total.parse().ok()
}
