// The library exposes the JSON boundary used by the Go CLI and the stages
// behind it. Nothing here runs the solution under test.

mod cargo_features;
pub mod prepare;
mod protocol;

pub use cargo_features::FeatureSelection;
pub use protocol::handle_request;
