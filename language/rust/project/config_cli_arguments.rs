// Reads Cargo's documented --config KEY=VALUE|PATH request values.
// Never changes the request or maintains a list of compiler options.
use crate::blocked::Blocked;
use std::ffi::OsString;

pub(crate) fn values(arguments: &[OsString]) -> Result<Vec<String>, Blocked> {
    let mut selected = Vec::new();
    let mut arguments = arguments.iter();
    while let Some(argument) = arguments.next() {
        if argument == "--" {
            break;
        }
        if argument == "--config" {
            let value = arguments.next().ok_or_else(|| invalid("missing value"))?;
            let value = value
                .to_str()
                .ok_or_else(|| invalid("value is not Unicode"))?;
            selected.push(value.to_string());
            continue;
        }
        if let Some(value) = argument
            .to_str()
            .and_then(|value| value.strip_prefix("--config="))
        {
            selected.push(value.to_string());
        }
    }
    Ok(selected)
}

fn invalid(cause: &str) -> Blocked {
    Blocked::Unreadable {
        what: "Cargo --config".to_string(),
        cause: cause.to_string(),
    }
}
