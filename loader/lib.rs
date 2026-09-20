// Purpose: reads a compiled program into memory the solver can use.
// Never:   holds format-specific work, which lives in its own directory.
// In:      nothing, this file only names the parts
// Out:     the image types and both format readers
// Fails:   not applicable
pub mod image;
pub mod region;

#[path = "linux/elf.rs"]
pub mod elf;
#[path = "macos/macho.rs"]
pub mod macho;
#[path = "macos/slice.rs"]
pub mod slice;
#[path = "macos/permission.rs"]
pub mod macos_permission;
#[path = "linux/permission.rs"]
pub mod linux_permission;

pub use image::{Image, LoadError, Segment};
pub use region::install_regions;
