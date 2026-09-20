// Purpose: places the registers a witness named, in architecture order.
// Never:   gives a register the solver did not report an invented value.
// In:      the register names and values from a witness
// Out:     the argument values, in the order the caller gave
// Fails:   not applicable, an unnamed register is zero

/// Reads the argument registers out of a witness, in the caller's order.
///
/// The caller names the registers its architecture passes arguments in, so
/// this file holds no list of its own. A register the witness does not name
/// is zero, which is what an unread register holds.
pub fn argument_values(named: &[(String, u64)], order: &[&str]) -> Vec<u64> {
    let mut values = vec![0u64; order.len()];
    for (name, value) in named {
        if let Some(index) = order.iter().position(|register| register == name) {
            values[index] = *value;
        }
    }
    values
}
