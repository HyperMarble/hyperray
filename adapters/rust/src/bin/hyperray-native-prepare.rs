// Expose native checker preparation through the public Rust API and JSON.
// An unsuccessful build must return an error, not an execution or proof result.
use hyperray_rust::prepare::{build, Request};
use std::io;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let request: Request = serde_json::from_reader(io::stdin().lock())?;
    let prepared = build(&request).map_err(io::Error::other)?;
    serde_json::to_writer_pretty(io::stdout().lock(), &prepared)?;
    Ok(())
}
