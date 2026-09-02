// A unified diff split into files and hunks. Input is the patch text alone;
// this file never opens the repository.

use super::hunk::Hunk;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct FileChange {
    pub path: String,
    pub hunks: Vec<Hunk>,
}

impl FileChange {
    pub fn added_lines(&self) -> usize {
        self.hunks.iter().map(|hunk| hunk.added.len()).sum()
    }

    pub fn removed_lines(&self) -> usize {
        self.hunks.iter().map(|hunk| hunk.removed.len()).sum()
    }
}

pub fn parse(patch: &str) -> Vec<FileChange> {
    let mut files: Vec<FileChange> = Vec::new();
    let mut in_hunks = false;
    for line in patch.lines() {
        if line.starts_with("diff ") || line.starts_with("--- ") {
            in_hunks = false;
        } else if let Some(path) = line.strip_prefix("+++ ") {
            files.push(FileChange {
                path: strip_diff_prefix(path),
                hunks: Vec::new(),
            });
            in_hunks = true;
        } else if let Some(file) = files.last_mut().filter(|_| in_hunks) {
            push_hunk_line(file, line);
        }
    }
    files
}

fn push_hunk_line(file: &mut FileChange, line: &str) {
    if let Some(hunk) = Hunk::from_header(line) {
        file.hunks.push(hunk);
    } else if let Some(hunk) = file.hunks.last_mut() {
        hunk.push_line(line);
    }
}

// Git writes `b/src/lib.rs`; the `b/` is diff syntax, not a directory.
fn strip_diff_prefix(path: &str) -> String {
    path.strip_prefix("b/").unwrap_or(path).to_string()
}
