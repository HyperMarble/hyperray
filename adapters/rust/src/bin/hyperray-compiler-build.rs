// This command reads one strict JSON build request and writes one build result.
// It never accepts process status or artifact hashes from the request.

use hyperray_rust::inventory::{build, BuildRequest};
use std::env;
use std::fs;
use std::path::PathBuf;

fn main() {
    let result = run(env::args().collect());
    match result {
        Ok(output) => println!("{output}"),
        Err(error) => {
            eprintln!("{error}");
            std::process::exit(1);
        }
    }
}

fn run(arguments: Vec<String>) -> Result<String, String> {
    let request_path = request_path(&arguments)?;
    let bytes = fs::read(&request_path)
        .map_err(|error| format!("read {}: {error}", request_path.display()))?;
    let request: BuildRequest =
        serde_json::from_slice(&bytes).map_err(|error| format!("decode request: {error}"))?;
    let result = build(&request).map_err(|error| error.to_string())?;
    serde_json::to_string_pretty(&result).map_err(|error| format!("encode result: {error}"))
}

fn request_path(arguments: &[String]) -> Result<PathBuf, String> {
    if arguments.len() == 3 && arguments[1] == "--request" {
        return Ok(PathBuf::from(&arguments[2]));
    }
    Err("usage: hyperray-compiler-build --request REQUEST.json".to_string())
}
