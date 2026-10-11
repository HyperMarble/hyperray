// Purpose: groups tool identity, hashing, commands, and machine facts.
// Never: treats a command name as an executable's identity.
pub mod c_toolchain;
pub mod digest;
pub mod environment;
pub mod native_argument;
pub mod os;
pub mod process_evidence;
pub mod run;
pub mod toolchain;
