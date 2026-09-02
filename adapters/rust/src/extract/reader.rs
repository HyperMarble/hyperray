// Every file EXTRACT pass 2 opens, with the reason. A read without a
// record here is a bug; the list is the audit of what the stage touched.

use serde::Serialize;
use std::path::{Path, PathBuf};

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
pub struct Opened {
    pub path: String,
    pub reason: String,
}

#[derive(Debug, Default)]
pub struct Reader {
    root: PathBuf,
    opened: Vec<Opened>,
}

impl Reader {
    pub fn new(root: &Path) -> Reader {
        Reader {
            root: root.to_path_buf(),
            opened: Vec::new(),
        }
    }

    pub fn read(&mut self, relative: &str, reason: &str) -> std::io::Result<String> {
        self.opened.push(Opened {
            path: relative.to_string(),
            reason: reason.to_string(),
        });
        std::fs::read_to_string(self.root.join(relative))
    }

    pub fn opened(&self) -> &[Opened] {
        &self.opened
    }
}
