// Root-facts source cases exercise compiler ABI facts directly.
// They never claim that any function is a valid machine root.

#[track_caller]
pub fn tracked(value: u8) -> u128 {
    value as u128
}

pub extern "C" fn boundary(value: bool, number: u128, _text: &str, _character: char) -> u128 {
    tracked(number as u8) + number + u128::from(value)
}
