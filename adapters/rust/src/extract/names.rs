// Function names a patch defines, and the hunk header context each hunk
// sits in. It reads patch text only; nothing here resolves a name.

use super::hunk::Hunk;

// The identifier after `fn ` in an added line, when the line is a definition.
pub fn defined_function(line: &str) -> Option<&str> {
    let rest = strip_visibility(line.trim_start());
    let rest = rest.strip_prefix("async ").unwrap_or(rest);
    let rest = rest.strip_prefix("unsafe ").unwrap_or(rest);
    let rest = rest.strip_prefix("const ").unwrap_or(rest);
    let after_fn = rest.strip_prefix("fn ")?;
    let end = after_fn.find(['(', '<'])?;
    let name = &after_fn[..end];
    is_identifier(name).then_some(name)
}

pub fn defined_in(hunk: &Hunk) -> Vec<String> {
    hunk.added
        .iter()
        .filter_map(|line| defined_function(line))
        .map(str::to_string)
        .collect()
}

// `pub`, `pub(crate)`, `pub(super)`, `pub(in path)`; the scope has no space
// after `pub`, so it is cut before the whitespace is.
fn strip_visibility(text: &str) -> &str {
    let Some(rest) = text.strip_prefix("pub") else {
        return text;
    };
    let rest = match rest.strip_prefix('(') {
        Some(scoped) => scoped.split_once(')').map_or(rest, |(_, tail)| tail),
        None => rest,
    };
    rest.trim_start()
}

fn is_identifier(name: &str) -> bool {
    let mut chars = name.chars();
    let first_ok = chars.next().is_some_and(|c| c.is_alphabetic() || c == '_');
    first_ok && chars.all(|c| c.is_alphanumeric() || c == '_')
}
