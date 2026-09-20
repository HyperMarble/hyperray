// Places instruction bytes in memory this processor will execute.
// The page must never stay writable while it is executable.
use crate::witness::RecheckError;

/// Memory holding instructions the processor can run.
pub struct ExecutablePage {
    address: *mut u8,
    size: usize,
}

impl ExecutablePage {
    pub fn address(&self) -> *const u8 {
        self.address
    }
}

impl Drop for ExecutablePage {
    fn drop(&mut self) {
        // Safety: the address came from mmap with this exact size.
        unsafe { libc::munmap(self.address as *mut libc::c_void, self.size) };
    }
}

/// Copies the instructions into memory and marks that memory executable.
///
/// The page is written first and made executable afterwards, because this
/// platform refuses a page that is both at once.
pub fn executable_page(instructions: &[u32]) -> Result<ExecutablePage, RecheckError> {
    let size = std::mem::size_of_val(instructions);
    if size == 0 {
        return Err(RecheckError::Unavailable("no instructions to run".to_string()));
    }
    let address = reserve(size)?;
    // Safety: the reservation is at least `size` bytes and is writable.
    unsafe { std::ptr::copy_nonoverlapping(instructions.as_ptr() as *const u8, address, size) };
    protect(address, size)?;
    Ok(ExecutablePage { address, size })
}

fn reserve(size: usize) -> Result<*mut u8, RecheckError> {
    // Safety: a null hint asks the system to choose the address.
    let address = unsafe {
        libc::mmap(
            std::ptr::null_mut(),
            size,
            libc::PROT_READ | libc::PROT_WRITE,
            libc::MAP_PRIVATE | libc::MAP_ANON,
            -1,
            0,
        )
    };
    if address == libc::MAP_FAILED {
        return Err(RecheckError::Unavailable("memory could not be reserved".to_string()));
    }
    Ok(address as *mut u8)
}

fn protect(address: *mut u8, size: usize) -> Result<(), RecheckError> {
    // Safety: the range came from the reservation above.
    let result = unsafe {
        libc::mprotect(address as *mut libc::c_void, size, libc::PROT_READ | libc::PROT_EXEC)
    };
    if result != 0 {
        return Err(RecheckError::Unavailable("memory could not be made executable".to_string()));
    }
    Ok(())
}
