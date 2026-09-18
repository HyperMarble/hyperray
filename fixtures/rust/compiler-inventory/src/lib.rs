#![no_std]
#![feature(thread_local)]

// The fixture keeps generic instances, cleanup paths, thread-local data, repeat values, and static data.

#[used]
#[no_mangle]
pub static RETAINED_VALUE: u64 = 17;

#[inline(never)]
fn repeated<T: Into<u64>>(value: T) -> u64 {
    value.into()
}

#[inline(never)]
fn repeated_array(value: u64) -> u64 {
    let values = [value; 4];
    values[2]
}

struct CleanupGuard(u64);

impl Drop for CleanupGuard {
    fn drop(&mut self) {
        self.0 = compiler_inventory_dependency_alias::dependency_value();
    }
}

#[inline(never)]
fn cleanup_case(value: u64) -> u64 {
    let guard = CleanupGuard(value);
    let result = retained_export(value);
    drop(guard);
    result
}

#[cfg(hyperray_thread_local)]
#[thread_local]
static THREAD_LOCAL_VALUE: u64 = 5;

#[cfg(hyperray_thread_local)]
#[inline(never)]
fn thread_local_value() -> u64 {
    THREAD_LOCAL_VALUE
}

#[inline(never)]
pub fn retained_export(value: u64) -> u64 {
    repeated(value)
        + repeated_array(value)
        + compiler_inventory_dependency_alias::dependency_value()
}

#[no_mangle]
pub extern "C" fn compiler_inventory_entry(value: u64) -> u64 {
    let thread_value = thread_local_value_if_enabled();
    retained_export(value) + repeated(value as u8) + cleanup_case(thread_value)
}

#[cfg(hyperray_thread_local)]
fn thread_local_value_if_enabled() -> u64 {
    thread_local_value()
}

#[cfg(not(hyperray_thread_local))]
fn thread_local_value_if_enabled() -> u64 {
    0
}
