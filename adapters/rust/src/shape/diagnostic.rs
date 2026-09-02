// The parts of a cargo `--message-format=json` line this stage reads.
// Shape from the Cargo book (external-tools, "JSON messages") and the
// rustc book (json.html). Fields not listed are ignored on purpose.

use serde::Deserialize;

#[derive(Deserialize)]
pub struct Line {
    pub reason: String,
    pub message: Option<Message>,
}

#[derive(Deserialize)]
pub struct Message {
    pub message: String,
    pub code: Option<Code>,
    pub spans: Vec<Span>,
}

#[derive(Deserialize)]
pub struct Code {
    pub code: String,
}

#[derive(Deserialize)]
pub struct Span {
    pub file_name: String,
    pub line_start: u32,
    pub line_end: u32,
    pub is_primary: bool,
}
