// Atomic shared-memory fixture for explicit Isla machine threads.
#![no_std]

use core::sync::atomic::{AtomicU64, Ordering};

#[no_mangle]
pub static SHARED: AtomicU64 = AtomicU64::new(0);

#[no_mangle]
pub extern "C" fn writer() -> u64 {
    SHARED.store(1, Ordering::SeqCst);
    0
}

#[no_mangle]
pub extern "C" fn reader() -> u64 {
    SHARED.load(Ordering::SeqCst)
}

#[no_mangle]
pub extern "C" fn _start() -> u64 {
    writer()
}
