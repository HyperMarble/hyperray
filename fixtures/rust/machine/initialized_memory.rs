// A shared value must retain the last store in this single-thread program.
// The compiler must retain both memory operations.
#![no_std]

use core::sync::atomic::{AtomicU64, Ordering};

static VALUE: AtomicU64 = AtomicU64::new(11);

#[no_mangle]
pub extern "C" fn _start(value: u64) -> u64 {
    if value == 0 {
        return VALUE.load(Ordering::Relaxed);
    }
    VALUE.store(value, Ordering::Relaxed);
    VALUE.load(Ordering::Relaxed)
}
