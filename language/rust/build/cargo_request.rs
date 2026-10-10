// Purpose: passes complete compilation requests to Cargo's own parser.
// Never: maintains an adapter list of accepted compiler or Cargo flags.
use std::ffi::OsString;
use std::path::PathBuf;

/// Native arguments for each Cargo command used to collect the build record.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct Arguments {
    pub build: Vec<OsString>,
    pub metadata: Vec<OsString>,
    pub doctests: Vec<OsString>,
}

#[derive(Clone, Debug, PartialEq)]
pub enum CompileCommand {
    Build,
    Rustc,
}

impl CompileCommand {
    pub fn name(&self) -> &'static str {
        match self {
            Self::Build => "build",
            Self::Rustc => "rustc",
        }
    }
}

#[derive(Clone, Debug, PartialEq)]
pub struct Request {
    pub executable: PathBuf,
    pub toolchain: Option<OsString>,
    pub directory: PathBuf,
    pub command: CompileCommand,
    pub arguments: Vec<OsString>,
}

impl Request {
    pub fn arguments(&self) -> Vec<OsString> {
        let mut arguments = Vec::new();
        if let Some(toolchain) = &self.toolchain {
            let mut selection = OsString::from("+");
            selection.push(toolchain);
            arguments.push(selection);
        }
        arguments.push(OsString::from(self.command.name()));
        arguments.extend(self.arguments.iter().cloned());
        arguments
    }
}
