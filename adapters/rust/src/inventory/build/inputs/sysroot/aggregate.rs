// Build the canonical aggregate for one measured sysroot input set.

use crate::inventory::{SysrootFileIdentity, ToolIdentity};
use sha2::{Digest, Sha256};
use std::path::Path;

pub(super) fn digest(
    root: &Path,
    target: &str,
    toolchain: &ToolIdentity,
    files: &[SysrootFileIdentity],
) -> String {
    let mut bytes = Vec::new();
    record(&mut bytes, &root.display().to_string());
    record(&mut bytes, target);
    record(&mut bytes, &toolchain.path.display().to_string());
    record(&mut bytes, &toolchain.sha256);
    for file in files {
        record(&mut bytes, &file.path.display().to_string());
        record(&mut bytes, &file.size.to_string());
        record(&mut bytes, &file.sha256);
    }
    let mut hasher = Sha256::new();
    hasher.update(bytes);
    format!("{:x}", hasher.finalize())
}

fn record(bytes: &mut Vec<u8>, value: &str) {
    bytes.extend(value.len().to_string().as_bytes());
    bytes.push(b':');
    bytes.extend(value.as_bytes());
    bytes.push(b'\n');
}
