// The library exposes the JSON boundary used by the Go CLI and the stages
// behind it. Nothing here runs the solution under test.

pub mod bound;
mod cargo_features;
pub mod extract;
pub mod inventory;
pub mod mir;
pub mod prepare;
mod protocol;
pub mod prove;
pub mod shape;

pub use cargo_features::FeatureSelection;
pub use protocol::handle_request;
