// These values preserve every loop that CBMC reports for a final GOTO model.
// Optional source fields stay optional because generated loops may omit them.

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct GotoLoop {
    pub name: String,
    #[serde(rename(deserialize = "sourceLocation"), alias = "source_location")]
    pub source_location: Option<GotoLocation>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct GotoLocation {
    pub file: Option<String>,
    pub function: Option<String>,
    pub line: Option<String>,
    pub column: Option<String>,
    #[serde(default, rename(deserialize = "pragma"), alias = "pragmas")]
    pub pragmas: Vec<String>,
    #[serde(rename(deserialize = "workingDirectory"), alias = "working_directory")]
    pub working_directory: Option<String>,
}
