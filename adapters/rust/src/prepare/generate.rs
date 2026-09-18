// Generate ABI connections and declared bounds, not subject behavior.
// Templates must remain independent of fixture names and expected outputs.
use super::{input_bits, Request};
use std::fs;

const SEARCH: &str = include_str!("../../../../execution/native/search.pml");
const OBSERVE: &str = include_str!("../../../../execution/native/observe.c");
const BRIDGE: &str = include_str!("../../../../execution/native/bridge.h");
const REPLAY: &str = include_str!("../../../../execution/native/replay.c");

pub fn write(request: &Request) -> Result<(), String> {
    let binding = format!(
        include_str!("binding.rs.in"),
        subject = request.subject.function,
        requirement = request.requirement.function
    );
    let bounds = format!(
        "/* Declared inputs only. This file does not describe program behavior. */\n\
         #define INPUT_BITS {}\n#define INPUT_MIN UINT64_C({})\n#define INPUT_MAX UINT64_C({})\n",
        input_bits(request.limits.minimum, request.limits.maximum)?,
        request.limits.minimum,
        request.limits.maximum,
    );
    for (name, content) in [
        ("search.pml", SEARCH),
        ("observe.c", OBSERVE),
        ("bridge.h", BRIDGE),
        ("replay.c", REPLAY),
        ("binding.rs", &binding),
        ("bounds.h", &bounds),
    ] {
        fs::write(request.directory.join(name), content)
            .map_err(|error| format!("write {name}: {error}"))?;
    }
    let source = serde_json::to_vec_pretty(request).map_err(|error| error.to_string())?;
    fs::write(request.directory.join("request.json"), source).map_err(|error| error.to_string())
}
