// Zero and nonzero inputs take different division paths.
// The test must retain the instructions of the untaken path.
#![no_std]

#[no_mangle]
pub extern "C" fn _start(value: u64) -> u64 {
    if value == 0 {
        return 17;
    }
    64 / value
}
