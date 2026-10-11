// Purpose: says which of the files Cargo reports are handed over: the
//          project's own, and any dynamic library a program loads at run
//          time.
// Never:   decides by a file's name. Cargo's own message says whose crate
//          it is and what kind of file it made.
use crate::cargo_messages::is_local;
use serde_json::Value;

/// True when the file stays a file of its own that a program loads at run
/// time: Cargo reports it as a Rust dynamic library. Every other crate from
/// outside the project ends up inside the files that use it. This one does
/// not, so it is handed over whoever owns the crate.
pub fn is_loaded_at_run_time(message: &Value) -> bool {
    let Some(types) = message["target"]["crate_types"].as_array() else {
        return false;
    };
    types.contains(&Value::from("dylib"))
}

/// True when the files one build message reports are handed over.
pub fn is_handed_over(message: &Value) -> bool {
    is_local(message) || is_loaded_at_run_time(message)
}
