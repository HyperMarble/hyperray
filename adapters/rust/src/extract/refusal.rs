// Charon's log read back as refusals, each with the file and line it
// names. Nothing here is inferred: no `-->` location, no refusal.

use serde::Serialize;

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
pub struct Refusal {
    pub reason: String,
    pub path: String,
    pub line: u32,
}

// Rust's own lints share the `warning:` shape and are kept too; the
// reason text is what tells them apart.
pub fn refusals_in(log: &str) -> Vec<Refusal> {
    let mut found = Vec::new();
    let mut lines = log.lines();
    while let Some(line) = lines.next() {
        let Some(reason) = reason_of(line) else {
            continue;
        };
        if let Some((path, number)) = lines.next().and_then(location_of) {
            found.push(Refusal {
                reason: reason.to_string(),
                path,
                line: number,
            });
        }
    }
    found
}

fn reason_of(line: &str) -> Option<&str> {
    line.strip_prefix("warning: ")
        .or_else(|| line.strip_prefix("error: "))
}

fn location_of(line: &str) -> Option<(String, u32)> {
    let rest = line.trim_start().strip_prefix("--> ")?;
    let (path, tail) = rest.split_once(':')?;
    let number = tail.split(':').next()?.parse().ok()?;
    Some((path.to_string(), number))
}
