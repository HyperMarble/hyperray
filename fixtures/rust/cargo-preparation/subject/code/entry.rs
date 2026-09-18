// Add five without boost or nine with boost, modulo u64.
// The broken feature deliberately corrupts the result at input twenty-three.
include!(concat!(env!("OUT_DIR"), "/increment.rs"));

fn increment() -> u64 {
    if cfg!(feature = "boost") {
        return helper::add(BASE, 4);
    }
    BASE
}

pub fn solve(input: u64) -> u64 {
    let output = helper::add(input, increment());
    if cfg!(feature = "broken") && input == 23 {
        return helper::add(output, 1);
    }
    output
}
