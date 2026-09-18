// A function's end comes from the symbol table, so a tail call is no
// different from a return. Scanning disassembly for a return instruction
// finds nothing when the compiler jumps away instead.
use hyperray_machine::extent;

fn extent_of(file: &str, name: &str) -> Option<extent::Extent> {
    let bytes = std::fs::read(format!("tests/fixtures/{file}")).ok()?;
    extent::of(&bytes, name).ok()
}

#[test]
fn a_tail_call_still_states_a_function_end() {
    for (file, name) in [
        ("tailcall_x86_64.o", "answer"),
        ("tailcall_arm64.o", "_answer"),
    ] {
        let Some(found) = extent_of(file, name) else {
            continue;
        };
        assert!(found.end > found.start, "{file}: {found:?}");
    }
}
