// Purpose: turns a Cargo command that refused into the reason the record
//          gives, in Cargo's and the compiler's own words.
// Never:   shortens or rewrites what they said.
use crate::blocked::Blocked;
use crate::cargo_observation::Observation;
use serde_json::Value;

pub(super) fn refused(process: &Observation) -> Blocked {
    Blocked::ToolFailed {
        tool: "cargo".to_string(),
        output: refusal(process),
    }
}

/// Cargo's refusal of a build, with the compiler's own messages. With JSON
/// output Cargo puts each rendered message on stdout, so they are read back
/// from there and placed before what Cargo wrote to stderr.
pub(super) fn refusal(process: &Observation) -> String {
    let printed = String::from_utf8_lossy(&process.stdout);
    let mut output = String::new();
    for line in printed.lines().filter(|line| line.starts_with('{')) {
        let message = serde_json::from_str::<Value>(line).unwrap_or_default();
        output.push_str(message["message"]["rendered"].as_str().unwrap_or_default());
    }
    output.push_str(&String::from_utf8_lossy(&process.stderr));
    output
}
