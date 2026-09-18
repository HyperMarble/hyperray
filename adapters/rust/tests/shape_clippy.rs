// Stage 2 reports target Clippy policy without adding Hyperray policy.

mod common;
#[path = "shape_clippy/capture.rs"]
mod shape_clippy_capture;
#[path = "shape_clippy/location.rs"]
mod shape_clippy_location;
#[path = "shape_clippy/policy.rs"]
mod shape_clippy_policy;
#[path = "shape_clippy/span.rs"]
mod shape_clippy_span;
