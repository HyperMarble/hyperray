// Define wrapping addition for the fixture's unsigned arithmetic contract.
// This library must not examine the checker or its expected results.
pub fn add(left: u64, right: u64) -> u64 {
    left.wrapping_add(right)
}
