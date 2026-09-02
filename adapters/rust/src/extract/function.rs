// Every whole `fn` that overlaps a line range, found by brace depth. It
// never guesses a body: a signature with no `{` yields nothing.

use super::names::defined_function;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Function {
    pub name: String,
    pub start_line: u32,
    pub end_line: u32,
    pub text: String,
}

pub fn overlapping(file_text: &str, first: u32, last: u32) -> Vec<Function> {
    let lines: Vec<&str> = file_text.lines().collect();
    let mut found = Vec::new();
    let mut index = 0;
    while index < lines.len() {
        match function_at(&lines, index) {
            Some(function) if overlaps(&function, first, last) => {
                index = to_index(function.end_line).unwrap_or(index) + 1;
                found.push(function);
            }
            Some(function) => index = to_index(function.end_line).unwrap_or(index) + 1,
            None => index += 1,
        }
    }
    found
}

fn overlaps(function: &Function, first: u32, last: u32) -> bool {
    function.start_line <= last && function.end_line >= first
}

fn function_at(lines: &[&str], start: usize) -> Option<Function> {
    let name = defined_function(lines[start])?;
    let end = body_end(lines, start)?;
    Some(Function {
        name: name.to_string(),
        start_line: to_line_number(start),
        end_line: to_line_number(end),
        text: lines[start..=end].join("\n"),
    })
}

// The body ends where brace depth returns to zero after the first `{`.
fn body_end(lines: &[&str], start: usize) -> Option<usize> {
    let mut depth: i32 = 0;
    let mut seen_open = false;
    for (offset, text) in lines.iter().skip(start).enumerate() {
        if !seen_open && text.trim_end().ends_with(';') {
            return None;
        }
        depth += brace_delta(text);
        seen_open |= text.contains('{');
        if seen_open && depth == 0 {
            return Some(start + offset);
        }
    }
    None
}

fn brace_delta(text: &str) -> i32 {
    let opens = i32::try_from(text.matches('{').count()).unwrap_or(0);
    let closes = i32::try_from(text.matches('}').count()).unwrap_or(0);
    opens - closes
}

fn to_index(line: u32) -> Option<usize> {
    usize::try_from(line).ok()?.checked_sub(1)
}

fn to_line_number(index: usize) -> u32 {
    u32::try_from(index + 1).unwrap_or(u32::MAX)
}
