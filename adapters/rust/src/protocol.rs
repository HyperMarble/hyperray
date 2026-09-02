// This module converts one JSON request into one JSON response.
// It must return malformed input as an error instead of a plausible response.

use serde::{Deserialize, Serialize};

#[derive(Debug, Deserialize)]
struct Request {
    protocol_version: u32,
    action: String,
}

#[derive(Debug, Serialize)]
struct VersionResponse<'a> {
    status: &'a str,
    protocol_version: u32,
    adapter: &'a str,
    adapter_version: &'a str,
}

pub fn handle_request(input: &str) -> Result<String, serde_json::Error> {
    let request: Request = serde_json::from_str(input)?;
    let response = VersionResponse {
        status: "ok",
        protocol_version: request.protocol_version,
        adapter: "rust",
        adapter_version: env!("CARGO_PKG_VERSION"),
    };

    let _action = request.action;
    serde_json::to_string(&response)
}
