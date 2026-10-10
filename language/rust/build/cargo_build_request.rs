// Purpose: forms the exact Cargo build request for one selected output.
// Never: parses or filters owner-provided Cargo arguments.
use crate::cargo_request::Arguments;
use crate::cargo_request::{
    CompileCommand,
    Request, //
};
use crate::project::Project;
use std::ffi::OsString;
use std::path::Path;

pub fn request(project: &Project, output: Option<&Path>, supplied: &Arguments) -> Request {
    let mut arguments = vec![OsString::from("--locked")];
    arguments.extend(supplied.build.iter().cloned());
    if let Some(directory) = output {
        arguments.push(OsString::from("--target-dir"));
        arguments.push(directory.as_os_str().to_owned());
    }
    arguments.push(OsString::from("--message-format=json"));
    Request {
        executable: "cargo".into(),
        toolchain: None,
        directory: project.root.clone(),
        command: CompileCommand::Build,
        arguments,
    }
}
