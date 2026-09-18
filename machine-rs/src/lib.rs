// The machine path: read one binary, run it symbolically, report one verdict.
// It must report what the file and the engine say, never a substitute.
pub mod architecture;
pub mod engine;
pub mod extent;
pub mod footprint;
pub mod image;
pub mod loader;
pub mod permission;
pub mod region;
pub mod section;
pub mod trace_queue;
