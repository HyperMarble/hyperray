// Reads a compiled program into memory the solver can use.
// Shared code lives here; each file format lives in its own directory.
pub mod image;
pub mod region;

#[path = "linux/elf.rs"]
pub mod elf;
#[path = "macos/macho.rs"]
pub mod macho;

pub use image::{Image, LoadError, Segment};
pub use region::install_regions;
