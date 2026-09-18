// Add thirteen modulo u64, then invert bit one of the result.
// The broken entry deliberately changes the result at input twenty-three.
pub mod workflow {
    pub fn solve(input: u64) -> u64 {
        let mut values = Box::new([input, 13, 2]);
        let increment = values[1];
        let selected = &mut values[0];
        *selected = selected.wrapping_add(increment);
        values[0] ^ values[2]
    }

    pub fn broken(input: u64) -> u64 {
        let output = solve(input);
        if input == 23 {
            return output ^ 1;
        }
        output
    }
}
