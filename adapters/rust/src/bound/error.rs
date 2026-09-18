// Stage 3 errors identify the compiler data, tool, or artifact that failed.
// A partial proof model must never be returned as complete evidence.

use std::path::PathBuf;

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Error {
    Edge {
        item: String,
        from: usize,
        to: usize,
        block_count: usize,
    },
    Row {
        path: String,
        name: String,
        reason: String,
    },
    Tool {
        tool: String,
        reason: String,
    },
    Artifact {
        path: PathBuf,
        reason: String,
    },
    Json {
        origin: String,
        reason: String,
    },
    Model {
        reason: String,
    },
}

impl std::fmt::Display for Error {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Edge {
                item,
                from,
                to,
                block_count,
            } => write!(
                formatter,
                "{item}: block {from} targets block {to}, but the body has {block_count} blocks"
            ),
            Self::Row { path, name, reason } => {
                write!(formatter, "{path}:{name}: {reason}")
            }
            Self::Tool { tool, reason } => write!(formatter, "{tool}: {reason}"),
            Self::Artifact { path, reason } => {
                write!(formatter, "{}: {reason}", path.display())
            }
            Self::Json { origin, reason } => write!(formatter, "{origin}: {reason}"),
            Self::Model { reason } => formatter.write_str(reason),
        }
    }
}

impl std::error::Error for Error {}
