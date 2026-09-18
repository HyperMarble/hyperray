// Compiler build selection has explicit public inputs and live integration gates.
// Missing live-tool configuration cannot silently count as a passed integration.
#[path = "compiler_selection/cases.rs"]
mod cases;
#[path = "compiler_selection/empty.rs"]
mod empty;
#[path = "compiler_selection/evidence.rs"]
mod evidence;
#[path = "compiler_selection/live.rs"]
mod live;
#[path = "compiler_selection/request.rs"]
mod request;
