// State the requested arithmetic independently from the subject and build script.
// These requirements must not call the subject or read its generated constant.
pub fn five(input: u64, output: u64) -> bool {
    output == input.wrapping_add(5)
}

pub fn nine(input: u64, output: u64) -> bool {
    output == input.wrapping_add(9)
}

pub fn bad_signature(input: u64) -> u64 {
    input
}
