// Reads a unified diff and returns the added line ranges per file.
// It never reads the repository. The diff alone decides what changed.

use std::ops::RangeInclusive;

#[derive(Debug, PartialEq, Eq)]
pub struct ChangedFile {
    pub path: String,
    pub added: Vec<RangeInclusive<usize>>,
}

pub fn changed_files(diff: &str) -> Vec<ChangedFile> {
    let mut files: Vec<ChangedFile> = Vec::new();
    let mut new_line = 0usize;

    for line in diff.lines() {
        if let Some(path) = line.strip_prefix("+++ b/") {
            files.push(ChangedFile {
                path: path.to_string(),
                added: Vec::new(),
            });
        } else if let Some(start) = hunk_new_start(line) {
            new_line = start;
        } else if let Some(file) = files.last_mut() {
            new_line = advance(file, line, new_line);
        }
    }

    files
}

fn hunk_new_start(line: &str) -> Option<usize> {
    let rest = line.strip_prefix("@@ ")?;
    let new_part = rest.split(' ').nth(1)?.strip_prefix('+')?;
    new_part.split(',').next()?.parse().ok()
}

fn advance(file: &mut ChangedFile, line: &str, new_line: usize) -> usize {
    match line.chars().next() {
        Some('+') => {
            record_added(file, new_line);
            new_line + 1
        }
        Some('-') => new_line,
        Some('\\') => new_line,
        _ => new_line + 1,
    }
}

fn record_added(file: &mut ChangedFile, line: usize) {
    match file.added.last_mut() {
        Some(range) if *range.end() + 1 == line => {
            *range = *range.start()..=line;
        }
        _ => file.added.push(line..=line),
    }
}
