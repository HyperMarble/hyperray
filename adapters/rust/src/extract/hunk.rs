// One hunk of a unified diff: the header ranges and the changed lines.
// It reads text only; it never touches the repository.

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Hunk {
    pub old_start: u32,
    pub old_count: u32,
    pub new_start: u32,
    pub new_count: u32,
    pub context: String,
    pub added: Vec<String>,
    pub removed: Vec<String>,
}

impl Hunk {
    pub fn from_header(header: &str) -> Option<Hunk> {
        let rest = header.strip_prefix("@@ -")?;
        let (old, rest) = rest.split_once(" +")?;
        let (new, context) = rest.split_once(" @@")?;
        let (old_start, old_count) = parse_range(old)?;
        let (new_start, new_count) = parse_range(new)?;
        Some(Hunk {
            old_start,
            old_count,
            new_start,
            new_count,
            context: context.trim().to_string(),
            added: Vec::new(),
            removed: Vec::new(),
        })
    }

    pub fn push_line(&mut self, line: &str) {
        if let Some(text) = line.strip_prefix('+') {
            self.added.push(text.to_string());
        } else if let Some(text) = line.strip_prefix('-') {
            self.removed.push(text.to_string());
        }
    }

    pub fn added_range(&self) -> (u32, u32) {
        (
            self.new_start,
            self.new_start + self.new_count.saturating_sub(1),
        )
    }
}

// A diff range is `start,count`, or just `start` when count is 1.
fn parse_range(text: &str) -> Option<(u32, u32)> {
    match text.split_once(',') {
        Some((start, count)) => Some((start.parse().ok()?, count.parse().ok()?)),
        None => Some((text.parse().ok()?, 1)),
    }
}
