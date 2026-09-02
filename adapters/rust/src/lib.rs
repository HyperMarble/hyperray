// The library exposes the JSON boundary used by the Go CLI and the stages
// behind it. Nothing here runs the solution under test.

pub mod extract;
mod protocol;
pub mod prove;
pub mod shape;

pub use protocol::handle_request;
