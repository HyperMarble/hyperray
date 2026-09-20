// Purpose: the image every loader returns, whatever format it read.
// Never:   holds a field that means one thing on macOS and another on Linux.
// In:      nothing, this file only declares the shape
// Out:     Image, Segment, LoadError
// Fails:   not applicable

/// One mapped range of a program, with the bytes that belong in it.
#[derive(Debug, PartialEq)]
pub struct Segment {
    pub name: String,
    pub address: u64,
    pub readable: bool,
    pub writable: bool,
    pub executable: bool,
    /// The size the range occupies in memory, which can exceed the bytes
    /// the file supplies. The remainder is zero.
    pub mapped_size: u64,
    pub bytes: Vec<u8>,
}

/// A program read from a file and ready to place in memory.
#[derive(Debug, PartialEq)]
pub struct Image {
    /// Where execution starts, when the file declares a start.
    ///
    /// A library declares none, so the caller names the function to prove
    /// instead of starting at the beginning.
    pub entry: Option<u64>,
    pub segments: Vec<Segment>,
}

#[derive(Debug, PartialEq)]
pub enum LoadError {
    Parse(String),
    UnsupportedArchitecture { cpu: u32, subtype: u32 },
    SegmentOutsideFile { name: String },
    ExtentOverflow { name: String },
}
