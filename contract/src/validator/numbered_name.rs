// Purpose: Recognize fixed role and variable names with positive decimal suffixes.
// Never: Parse a suffix into a machine integer or accept zero and leading zeros.
// In: One symbol and one fixed prefix.
// Out: True only for prefix followed by a positive decimal integer.
// Fails: This function reports invalid text as false.

pub(crate) fn has_positive_suffix(name: &str, prefix: &str) -> bool {
    let Some(digits) = name.strip_prefix(prefix) else {
        return false;
    };
    let Some(first) = digits.as_bytes().first() else {
        return false;
    };
    first.is_ascii_digit()
        && *first != b'0'
        && digits.as_bytes()[1..].iter().all(u8::is_ascii_digit)
}
