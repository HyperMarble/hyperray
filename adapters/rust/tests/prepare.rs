// Public preparation tests exercise request validation without private construction.
// No missing external tool can silently skip these decisions.
#[path = "prepare/bounds.rs"]
mod bounds;
#[path = "prepare/cargo.rs"]
mod cargo;
#[path = "prepare/fixture.rs"]
mod fixture;
#[path = "prepare/request.rs"]
mod request;
