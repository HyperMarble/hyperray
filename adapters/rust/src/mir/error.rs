// Errors at the MIR JSON boundary. A schema mismatch must never look like
// an empty compiler result.

#[derive(Debug)]
pub enum ReadError {
    Json(serde_json::Error),
    Schema { expected: u32, found: u32 },
}

impl std::fmt::Display for ReadError {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Json(error) => write!(formatter, "invalid MIR JSON: {error}"),
            Self::Schema { expected, found } => {
                write!(formatter, "MIR schema {found} is not schema {expected}")
            }
        }
    }
}

impl std::error::Error for ReadError {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match self {
            Self::Json(error) => Some(error),
            Self::Schema { .. } => None,
        }
    }
}
