// Preserve the earlier experiment's independently stated result contracts.
// These functions must not call a subject or depend on observed outputs.
pub fn stack_array(input: u64, output: u64) -> bool {
    let index = input % 4;
    if index == 3 {
        return output == 9;
    }
    output == input + index + 9
}

pub fn mixed_width(input: u64, output: u64) -> bool {
    output == input
}

pub fn recursive_calls(input: u64, output: u64) -> bool {
    let depth = input % 4;
    output == depth * (depth + 1) / 2
}

pub fn dynamic_call(input: u64, output: u64) -> bool {
    output == input + 7
}

pub fn generic_calls(input: u64, output: u64) -> bool {
    output == 2 * input + 10
}

pub fn combined(input: u64, output: u64) -> bool {
    output == input + 15
}

pub fn transformed(input: u64, output: u64) -> bool {
    output == (input.wrapping_add(13) ^ 2)
}

pub fn bad_signature(input: u64) -> u64 {
    input
}
